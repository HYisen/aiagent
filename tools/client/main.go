package main

import (
	"aiagent/console"
	client2 "aiagent/tools/client/client"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
)

var endpoint = flag.String("endpoint", "http://localhost:8080/ai", "aiagent endpoint")

func main() {
	flag.Parse()
	client := client2.NewClient(*endpoint)
	handler := NewChatLineHandler(client)
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}

type ChatLineHandler struct {
	client      *client2.Client
	initialized bool
	sessionID   int
}

func NewChatLineHandler(client *client2.Client) *ChatLineHandler {
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
