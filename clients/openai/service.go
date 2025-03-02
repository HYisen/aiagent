package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Service struct {
	baseURL string
	apiKey  string
}

func NewService(baseURL, apiKey string) *Service {
	return &Service{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (s *Service) chat(ctx context.Context, request RequestWhole) (body io.ReadCloser, err error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	url := s.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *Service) OneShot(ctx context.Context, request Request) (*ChatCompletion, error) {
	body, err := s.chat(ctx, RequestWhole{
		Request: request,
		Stream:  false,
	})
	if err != nil {
		return nil, err
	}
	defer CloseAndWarnIfFail(body)

	var response Response
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, err
	}
	return &response.ChatCompletion, nil
}

func CloseAndWarnIfFail(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Printf("warn: potential resource leak as failed to close body: %v", err)
	}
}

func (s *Service) OneShotStream(
	ctx context.Context,
	request Request,
	ch chan<- ChatCompletionChunk,
) (aggregated *ChatCompletion, err error) {
	body, err := s.chat(ctx, RequestWhole{
		Request: request,
		Stream:  true,
	})
	if err != nil {
		return nil, err
	}
	defer CloseAndWarnIfFail(body)

	scanner := bufio.NewScanner(body)
	var done bool
	// Initiate choices with len 1 as Aggregate does not create, don't ask me how I find it vital.
	aggregated = &ChatCompletion{Choices: make([]Choice, 1)}
	for scanner.Scan() {
		// I have tried pulling parseTrunkData to lower Cyclomatic Complexity.
		// Blaming the done control, it's pull 5 and leave extra 2 CC in place,
		// which helps little. In the end I decided to leave the complexity here.

		// The SSE date line and done check never works while I am developing their handler.
		// Comparing to len(choices), they are more likely to change on the server side.
		// Maybe I shall make them warnings once triggered,
		// implementing best-effort strategy in the cost of sensitivity.

		line := scanner.Text()
		// ref https://api-docs.deepseek.com/faq#why-are-empty-lines-continuously-returned-when-calling-the-api
		if line == ": keep-alive" {
			continue
		}
		// I also witness blank line in stream mode. Now that server has sent it,
		// we just ignore it rather than somehow yield gracefully like exponential backoff.
		if line == "" {
			continue
		}

		if done {
			return nil, fmt.Errorf("extra line after [DONE] %q", line)
		}

		after, found := strings.CutPrefix(line, "data: ")
		if !found {
			return nil, fmt.Errorf("bad format SSE data line %q", line)
		}
		if after == "[DONE]" {
			done = true
			continue
		}

		var response ChunkResponse
		if err := json.Unmarshal([]byte(after), &response); err != nil {
			return nil, err
		}
		ch <- response.ChatCompletionChunk
		aggregated.Aggregate(response.ChatCompletionChunk)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return aggregated, nil
}
