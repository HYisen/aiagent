package openai

import (
	"bufio"
	"bytes"
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

func (s *Service) OneShot(content string) error {
	data, err := json.Marshal(Request{
		Messages: []Message{{
			Role:    "user",
			Content: content,
		}},
		Model: ChatModelDeepSeekR1,
	})
	if err != nil {
		return err
	}

	url := s.baseURL + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("warn: potential resource leak as failed to close body: %v", err)
		}
	}(resp.Body)

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	fmt.Printf("%+v\n", response)
	return nil
}

func (s *Service) OneShotStream(content string) error {
	data, err := json.Marshal(Request{
		Messages: []Message{{
			Role:    "user",
			Content: content,
		}},
		Model:  ChatModelDeepSeekR1,
		Stream: true,
	})
	if err != nil {
		return err
	}

	url := s.baseURL + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("warn: potential resource leak as failed to close body: %v", err)
		}
	}(resp.Body)

	scanner := bufio.NewScanner(resp.Body)
	var done bool
	for scanner.Scan() {
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
			return fmt.Errorf("extra line after [DONE] %q", line)
		}

		after, found := strings.CutPrefix(line, "data: ")
		if !found {
			return fmt.Errorf("bad format SSE data line %q", line)
		}
		if after == "[DONE]" {
			done = true
			continue
		}

		var response ChunkResponse
		if err := json.Unmarshal([]byte(after), &response); err != nil {
			return err
		}
		fmt.Printf("%+v\n", response)
	}
	return scanner.Err()
}
