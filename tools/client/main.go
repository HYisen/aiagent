package main

import (
	"aiagent/clients/openai"
	"aiagent/console"
	"aiagent/service/chat"
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

type Client struct {
	endpoint string
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint}
}

func (c *Client) CreateSession() (id int, error error) {
	resp, err := http.Post(fmt.Sprintf("%s/v1/sessions", c.endpoint), "", nil)
	if err != nil {
		return 0, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	num, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}
	return num, nil
}

func (c *Client) Chat(sessionID int, content string) (words <-chan string, err error) {
	url := fmt.Sprintf("%s/v1/sessions/%d/chat?stream=true", c.endpoint, sessionID)
	req := &chat.RequestPayload{
		Content: content,
		Model:   openai.ChatModelDeepSeekR1,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, wf.JSONContentType, bytes.NewReader(data))
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
