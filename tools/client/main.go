package main

import (
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/console"
	"aiagent/service/chat"
	"aiagent/tools/client/keeper"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/hyisen/wf"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var endpoint = flag.String("endpoint", "http://localhost:8640", "aiagent endpoint")

func main() {
	flag.Parse()
	client := NewClient(*endpoint)
	handler := NewChatLineHandler(client)
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}

type ChatLineHandler struct {
	client      *Client
	initialized bool
	sessionID   int
}

func NewChatLineHandler(client *Client) *ChatLineHandler {
	return &ChatLineHandler{client: client}
}

const initLinePrefix = "init"

func checkAndParseInitLine(s string) (isInitLine bool, createSession bool, oldSessionID int) {
	rest, found := strings.CutPrefix(s, initLinePrefix)
	if !found {
		return false, false, 0
	}
	if rest == "" {
		return true, true, 0
	}
	id, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		return false, false, 0
	}
	return true, false, id
}

func (h *ChatLineHandler) HandleLine(line string) {
	isInitLine, createSession, id := checkAndParseInitLine(line)
	if !isInitLine && !h.initialized {
		fmt.Printf(`Type "%s" to initialize.
Type "%s 4" to continue session ID 4\n`, initLinePrefix, initLinePrefix)
		return
	}
	if isInitLine {
		if createSession {
			fmt.Println("connecting...")
			id, err := h.client.CreateSession()
			if err != nil {
				log.Fatal(err)
			}
			h.sessionID = id
			fmt.Printf("initialized to session id %d\n", h.sessionID)
		} else {
			h.sessionID = id
			fmt.Printf("try continue on session id %d\n", h.sessionID)
		}
		h.initialized = true
		return
	}

	words, err := h.client.Chat(h.sessionID, line)
	if err != nil {
		log.Fatal(err)
	}
	for word := range words {
		fmt.Print(word)
	}
}

type TokenProvider interface {
	GetToken() (string, error)
}

type Client struct {
	endpoint      string
	apiPathPrefix string
	tokenProvider TokenProvider
}

const skipToken = ""

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint:      endpoint,
		apiPathPrefix: "v1",
		tokenProvider: keeper.NewLocalKeeper(skipToken),
	}
}

// TryAttachToken add token header to req with token from c.tokenProvider, if it's not skipToken.
// This behaviour shall be privileged and limited, thus I drop the other path that
// feature Client with a http.Client which has http.RoundTripper that automatically gets and attaches token.
func (c *Client) TryAttachToken(req *http.Request) {
	token, err := c.tokenProvider.GetToken()
	if err != nil {
		log.Fatal(err)
	}
	if token != skipToken {
		req.Header.Set("Token", token)
	}
}

func (c *Client) isV1() bool {
	// TODO: later, use seperated Clients to avoid achieving polymorphism through switch by a variable.
	return c.apiPathPrefix == "v1"
}

func (c *Client) CreateSession() (id int, error error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s/sessions", c.endpoint, c.apiPathPrefix), nil)
	c.TryAttachToken(req)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func(c io.Closer) {
		err = errors.Join(err, c.Close())
	}(resp.Body)

	if resp.StatusCode == http.StatusForbidden {
		c.upgrade()
		return c.CreateSession()
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected response status code %d: %s", resp.StatusCode, string(data))
	}

	if c.isV1() {
		num, err := strconv.Atoi(string(data))
		if err != nil {
			return 0, err
		}
		return num, nil
	} else {
		var session model.Session
		if err := json.Unmarshal(data, &session); err != nil {
			return 0, err
		}
		return session.ScopedID, nil
	}
}

func (c *Client) Chat(sessionID int, content string) (words <-chan string, err error) {
	url := fmt.Sprintf("%s/%s/sessions/%d/chat?stream=true", c.endpoint, c.apiPathPrefix, sessionID)
	payload := &chat.RequestPayload{
		Content: content,
		Model:   openai.ChatModelDeepSeekR1,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", wf.JSONContentType)
	c.TryAttachToken(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status %d %s", resp.StatusCode, resp.Status)
	}

	ch := make(chan string)
	// Pass owner of body to goroutine, DO NOT close it here. Don't ask me how I know it.
	go transform(resp.Body, ch)
	return ch, nil
}

func transform(body io.ReadCloser, output chan<- string) {
	defer func() {
		_ = body.Close()
		close(output)
	}()

	// The implementation here follows the guideline, in some way.
	// ref https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
	// Differences (no difference as my server don't use them)
	// - Assume there is always a nice space after :.
	// - Field name "id" and "retry" not supported.
	scanner := bufio.NewScanner(body)
	eventType := ""
	var data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ":") {
			continue
		}
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = value
			continue
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			// Not strings.Builder{} as there is typically none or little append.
			// Same to add LF on every line and remove the last after join.
			if data != "" {
				data += "\n"
			}
			data += value
			continue
		}
		if line == "" {
			output <- message(eventType, data)
			// DO DOT forget to clean buffer in the end of dispatch, don't ask me how I found it vital.
			eventType = ""
			data = ""
			continue
		}
		log.Fatal("unexpected line: ", line)
	}
	if err := scanner.Err(); err != nil {
		output <- fmt.Sprintf("\n err: %v", err)
	}
}

func message(eventType string, data string) (word string) {
	switch eventType {
	case "head":
		return data + "\n"
	case "role":
		return fmt.Sprintf("role = %s\n", data)
	case "cotEnd":
		return console.COTEndMessage()
	case "":
		return data
	case "finish":
		return fmt.Sprintf("\nFinishReason = %s\n", data)
	case "usage":
		return data + "\n"
	case "error":
		return fmt.Sprintf("\nserver error: %s\n", data)
	}
	log.Fatal(fmt.Errorf("message of eventType %s: %w", eventType, errors.ErrUnsupported))
	return "unreachable"
}
