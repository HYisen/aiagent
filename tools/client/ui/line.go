package ui

import (
	"aiagent/console"
	"aiagent/helpers/pricer"
	"aiagent/tools/client/clients/ai"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/width"
)

type SoftWrapOptions struct {
	TerminalWidth int
	WideCharScale float64
}

type MultiLineHelper interface {
	EnableMultiLineOnce()
	ExitMultiLineHint() string
}

type Handler struct {
	client ai.Client
	swo    SoftWrapOptions
	remote MultiLineHelper

	initialized bool
	sessionID   int
}

func NewHandler(client ai.Client, softWrapOptions SoftWrapOptions, remote MultiLineHelper) *Handler {
	return &Handler{client: client, swo: softWrapOptions, remote: remote}
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

func tryLoginOnceIfForbidden[ReturnType any](
	h *Handler,
	fn func(c ai.Client) (ReturnType, error),
) (ReturnType, error) {
	ret, errOne := fn(h.client)
	if errOne == nil {
		return ret, nil
	}
	if !errors.Is(errOne, ai.ErrForbidden) {
		return ret, errOne
	}

	// assert errors.Is(errOne, ai.ErrForbidden)
	neo, errTwo := h.client.UpgradeOptional()
	if errTwo != nil {
		return ret, errTwo
	}
	if neo == nil {
		return ret, errors.Join(errOne, errors.New("client does not support upgrade"))
	}
	h.client = neo
	return fn(h.client)
}

func localShortDateTime(epochMillis int64) string {
	return time.UnixMilli(epochMillis).Local().Format(time.UnixDate)
}

func PrintSessionTable[T ai.Session](sessions []T) {
	if len(sessions) == 0 {
		fmt.Println("Empty Session Table")
		return
	}
	fmt.Printf("%s\tRounds\tCreatedAt\tUpdatedAt\n", sessions[0].IDField())
	for _, s := range sessions {
		fmt.Printf(
			"%4d\t%4d\t%s\t%s\t%s\n",
			s.IDValue(),
			s.SessionCommon().Rounds,
			localShortDateTime(s.SessionCommon().CreateTimeEpochMilli),
			localShortDateTime(s.SessionCommon().UpdateTimeEpochMilli),
			s.SessionCommon().Name,
		)
	}
}

var commandLineToActions = map[string]func(h *Handler){
	":ls": func(h *Handler) {
		sessions, err := tryLoginOnceIfForbidden(h, func(c ai.Client) ([]ai.Session, error) {
			return c.ListSessions()
		})
		if err != nil {
			fmt.Printf("List Sessions failed: %v\n", err)
			return
		}
		slices.SortFunc(sessions, func(lhs ai.Session, rhs ai.Session) int {
			return int(lhs.SessionCommon().UpdateTimeEpochMilli - rhs.SessionCommon().UpdateTimeEpochMilli)
		})
		PrintSessionTable(sessions)
	},
	":version": func(h *Handler) {
		version, err := tryLoginOnceIfForbidden(h, func(c ai.Client) (*debug.BuildInfo, error) {
			return c.GetVersion()
		})
		if err != nil {
			fmt.Printf("Get Version failed: %v\n", err)
			return
		}
		fmt.Println(version)
	},
	":ml": func(h *Handler) {
		fmt.Println(h.remote.ExitMultiLineHint())
		h.remote.EnableMultiLineOnce()
	},
}

func (h *Handler) HandleInput(content string) {
	for cmd, action := range commandLineToActions {
		if content == cmd {
			action(h)
			return
		}
	}

	cmd, ok := strings.CutPrefix(content, ":gn ")
	if ok {
		scopedIDToNeoNameNullable, err := h.client.GenerateSessionName(cmd)
		if err != nil {
			fmt.Printf("Generate Session [%s] Name failed: %v\n", cmd, err)
			return
		}
		if scopedIDToNeoNameNullable != nil {
			for scopedID, neoName := range scopedIDToNeoNameNullable {
				fmt.Printf("%d => %s\n", scopedID, neoName)
			}
		}
		fmt.Println("done")
		return
	}

	isInitLine, createSession, id := checkAndParseInitLine(content)
	if !isInitLine && !h.initialized {
		fmt.Printf(`Type "%s" to initialize.
Type "%s 4" to continue session ID 4\n`, initLinePrefix, initLinePrefix)
		return
	}
	if isInitLine {
		if createSession {
			fmt.Println("connecting...")
			id, err := tryLoginOnceIfForbidden(h, func(c ai.Client) (int, error) {
				return c.CreateSession()
			})
			if err != nil {
				fmt.Printf("Create Session failed: %v\n", err)
				return
			}
			h.sessionID = id
			fmt.Printf("initialized to session id %d\n", h.sessionID)
		} else {
			h.sessionID = id
			fmt.Printf("try continue on session id %d\n", h.sessionID)
			session, err := h.client.GetSession(id)
			if err != nil {
				fmt.Printf("Get Session %d failed: %v\n", id, err)
				return
			}
			fmt.Printf("session name = %s\n", session.Name)
			for i, chat := range session.Chats {
				// SoftWrap not supported as a glance of history is enough, and it's non-trivial to implement.
				fmt.Printf("| #%4d %s\n", i, localShortDateTime(chat.CreateTime))
				PrintWithPrefix("  ", chat.Input)
				fmt.Printf("| model = %s FinishReason = %s\n", chat.Result.Model, chat.Result.FinishReason)
				PrintWithPrefix("  ", chat.Result.ReasoningContent)
				PrintWithPrefix("  ", console.COTEndMessage())
				PrintWithPrefix("  ", chat.Result.Content)
				usage := pricer.OpenAIUsage(chat.Result.ChatCompletion().Usage)
				fmt.Printf("| %s\n\n", pricer.PriceOrDefault(chat.Result.Model).Cost(usage))
			}
		}
		h.initialized = true
		return
	}

	words, err := h.client.Chat(h.sessionID, content)
	if err != nil {
		log.Fatal(err)
	}
	printWithSoftWrap(h.swo, words)
}

func PrintWithPrefix(linePrefix string, multiLine string) {
	for line := range strings.SplitSeq(multiLine, "\n") {
		fmt.Print(linePrefix)
		fmt.Println(line)
	}
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
