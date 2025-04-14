package ai

import (
	"aiagent/clients/openai"
	"aiagent/console"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

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
		return data + "\n" + CostMessage(data) + "\n"
	case "error":
		return fmt.Sprintf("\nserver error: %s\n", data)
	}
	log.Fatal(fmt.Errorf("message of eventType %s: %w", eventType, errors.ErrUnsupported))
	return "unreachable"
}

func CostMessage(data string) string {
	var line string
	var usage openai.Usage
	if err := json.Unmarshal([]byte(data), &usage); err != nil {
		line = fmt.Sprintf("err: %v", err)
	}
	line = "estimated cost " + NewDeepSeekReasonerPrice().Cost(OpenAIUsage(usage))
	return line
}
