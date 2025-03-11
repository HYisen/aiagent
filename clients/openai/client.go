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

type Client struct {
	baseURL string
	apiKey  string
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (c *Client) chat(ctx context.Context, request RequestWhole) (body io.ReadCloser, err error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) OneShot(ctx context.Context, request Request) (*ChatCompletion, error) {
	body, err := c.chat(ctx, RequestWhole{
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

// OneShotStream use ch as chunk output, and will close it when it's done.
// It uses one parameter as an output, because aggregated generates slowly.
// If put ch in output, considering first chunk may be tens of seconds earlier than aggregated,
// users would have to aggregate themselves.
func (c *Client) OneShotStream(
	ctx context.Context,
	request Request,
	ch chan<- ChatCompletionChunk,
) (aggregated *ChatCompletion, err error) {
	body, err := c.chat(ctx, RequestWhole{
		Request: request,
		Stream:  true,
	})
	if err != nil {
		return nil, err
	}
	defer CloseAndWarnIfFail(body)
	defer close(ch)

	scanner := bufio.NewScanner(body)
	scanner.Split(ScanDoubleNewLine)
	var done bool
	// Initiate choices with len 1 as Aggregate does not create, don't ask me how I find it vital.
	aggregated = &ChatCompletion{Choices: make([]Choice, 1)}
	for scanner.Scan() {
		// I have tried pulling parseTrunkData to lower Cyclomatic Complexity.
		// Blaming the done control, it's pull 5 and leave extra 2 CC in place,
		// which helps little. In the end I decided to leave the complexity here.

		// The prefix data: and done check never works while I am developing their handler.
		// Comparing to len(choices), they are more likely to change on the server side.
		// Maybe I shall make them warnings once triggered,
		// implementing best-effort strategy in the cost of sensitivity.

		line := scanner.Text()
		// ref https://api-docs.deepseek.com/faq#why-are-empty-lines-continuously-returned-when-calling-the-api
		// The ignore behaviour is also required by SSE, while other cases HasPrefix : are not respected.
		// ref https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream
		if line == ": keep-alive" {
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

// ScanDoubleNewLine is a split function for a [Scanner] that returns each event of text/event-stream,
// stripped of any trailing end-of-event marker. The returned line may be empty.
// The end-of-event marker is two newline. In regular expression notation, it is `\n\n`.
// The last non-empty blob of input will be returned even if it has no end-of-event marker.
//
// The end-of-event marker is actually an end-of-line marker of a normal line and an empty line which
// indicates dispatch the event in SSE.
//
// The function is copied from bufio.ScanLines and then modified. The differences are
// 1. no dropCR as it's LF rather than CRLF or anything else on the server side at present.
// 2. find index of "\n\n" rather than "\n", and return advance matches.
//
// ref https://html.spec.whatwg.org/multipage/server-sent-events.html#server-sent-events
func ScanDoubleNewLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\n\n"); i >= 0 {
		// We have a full newline-terminated line.
		return i + 2, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
