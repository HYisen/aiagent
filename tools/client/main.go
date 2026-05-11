package main

import (
	"aiagent/console"
	"aiagent/tools/client/clients/ai"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"
)

var endpoint = flag.String("endpoint", "https://hyisen.net/ai", "aiagent endpoint")

func main() {
	flag.Parse()
	handler := NewChatLineHandler(ai.NewClient(*endpoint))
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}

type ChatLineHandler struct {
	client      ai.Client
	initialized bool
	sessionID   int
}

func NewChatLineHandler(client ai.Client) *ChatLineHandler {
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

func (h *ChatLineHandler) createSession() (idOrScopedID int, err error) {
	id, errOne := h.client.CreateSession()
	if errOne == nil || !errors.Is(errOne, ai.ErrForbidden) {
		return id, errOne
	}
	neo, errTwo := h.client.UpgradeOptional()
	if errTwo != nil {
		return 0, errTwo
	}
	if neo == nil {
		return 0, errors.Join(errOne, errors.New("client does not support upgrade"))
	}
	h.client = neo
	return h.client.CreateSession()
}

// PrintVersion prints server version.
func (h *ChatLineHandler) PrintVersion() error {
	version, errOne := h.client.GetVersion()
	if errOne == nil {
		fmt.Println(version)
		return nil
	}
	if !errors.Is(errOne, ai.ErrForbidden) {
		return errOne
	}
	neo, errTwo := h.client.UpgradeOptional()
	if errTwo != nil {
		return errTwo
	}
	if neo == nil {
		return errors.Join(errOne, errors.New("client does not support upgrade"))
	}
	h.client = neo
	return h.PrintVersion()
}

func (h *ChatLineHandler) HandleLine(line string) {
	if line == ":ls" {
		idToName, err := h.client.ListSessions()
		if err != nil {
			fmt.Printf("server error: %v\n", err)
			return
		}
		for id, name := range idToName {
			fmt.Printf("%d\t%s\n", id, name)
		}
		return
	}
	if line == ":version" {
		if err := h.PrintVersion(); err != nil {
			slog.Error("PrintVersion failed", "err", err)
		}
		return
	}

	isInitLine, createSession, id := checkAndParseInitLine(line)
	if !isInitLine && !h.initialized {
		fmt.Printf(`Type "%s" to initialize.
Type "%s 4" to continue session ID 4\n`, initLinePrefix, initLinePrefix)
		return
	}
	if isInitLine {
		if createSession {
			fmt.Println("connecting...")
			id, err := h.createSession()
			if err != nil {
				fmt.Printf("Create session failed: %v\n", err)
				return
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
