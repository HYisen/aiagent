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

	"golang.org/x/text/width"
)

var endpoint = flag.String("endpoint", "https://hyisen.net/ai", "aiagent endpoint")
var softWrapWidth = flag.Int("softWrapWidth", 0, "soft wrap output line at width for unable terminals, 0 for disable")
var wideCharScale = flag.Float64("wideCharScale", 1.667, "one CJK wide char as how many ASCII chars in soft wrap")

func main() {
	flag.Parse()
	handler := NewChatLineHandler(ai.NewClient(*endpoint), SoftWrapOptions{
		TerminalWidth: *softWrapWidth,
		WideCharScale: *wideCharScale,
	})
	controller := console.NewController(handler, console.NewDefaultOptions())
	controller.Run()
}

type SoftWrapOptions struct {
	TerminalWidth int
	WideCharScale float64
}

type ChatLineHandler struct {
	client ai.Client
	swo    SoftWrapOptions

	initialized bool
	sessionID   int
}

func NewChatLineHandler(client ai.Client, softWrapOptions SoftWrapOptions) *ChatLineHandler {
	return &ChatLineHandler{client: client, swo: softWrapOptions}
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
		idToDesc, err := h.client.ListSessions()
		if err != nil {
			fmt.Printf("server error: %v\n", err)
			return
		}
		for id, desc := range idToDesc {
			fmt.Printf("%d\t%s\n", id, desc)
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
	printWithSoftWrap(h.swo, words)
}

func printWithSoftWrap(opts SoftWrapOptions, words <-chan string) {
	if opts.TerminalWidth == 0 {
		for word := range words {
			fmt.Print(word)
		}
		return
	}

	ttl := float64(opts.TerminalWidth)
	var buf strings.Builder
	for word := range words {
		for _, ch := range word {
			if ch == '\n' {
				fmt.Println(buf.String())
				buf.Reset()
				ttl = float64(opts.TerminalWidth)
				continue
			}

			var w float64
			if LooksWide(ch) {
				w = opts.WideCharScale
			} else {
				w = 1.0
			}

			ttl -= w
			if ttl < 0 {
				buf.WriteString("⏎")
				fmt.Println(buf.String())
				buf.Reset()
				buf.WriteRune(ch)
				ttl = float64(opts.TerminalWidth) - w
			} else {
				buf.WriteRune(ch)
			}
		}
	}
	if buf.Len() != 0 {
		panic(fmt.Errorf("unused soft-wrap buf %s", buf.String()))
	}
}

func LooksWide(r rune) bool {
	switch width.LookupRune(r).Kind() {
	case width.Neutral:
		fallthrough
	case width.EastAsianAmbiguous:
		fallthrough
	case width.EastAsianNarrow:
		fallthrough
	case width.EastAsianHalfwidth:
		return false
	case width.EastAsianWide:
		fallthrough
	case width.EastAsianFullwidth:
		fallthrough
	default:
		return true
	}
}
