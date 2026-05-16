package ui

import (
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

// tryLoginIfCanOrSelf checks input e, return input itself if it can not be solved through login,
// otherwise login and return nullable login error. When using this helper function,
// developers are suggested to check err nil and do happy path first (opposite to handle err in guard closure first).
// Use this to map non nil err, if err keeps non nil, handle err in guard closure and exit.
// Otherwise, as the err turns nil, everything is okay, do receiver method again.
func (h *ChatLineHandler) tryLoginIfCanOrSelf(e error) error {
	if e == nil || !errors.Is(e, ai.ErrForbidden) {
		return e
	}
	neo, err := h.client.UpgradeOptional()
	if err != nil {
		return err
	}
	if neo == nil {
		return errors.Join(e, errors.New("client does not support upgrade"))
	}
	h.client = neo
	return nil
}

func (h *ChatLineHandler) createSession() (idOrScopedID int, err error) {
	id, err := h.client.CreateSession()
	if err == nil {
		return id, nil
	}
	if err = h.tryLoginIfCanOrSelf(err); err != nil {
		return 0, err
	}
	return h.client.CreateSession()
}

func (h *ChatLineHandler) getVersion() (*debug.BuildInfo, error) {
	version, err := h.client.GetVersion()
	if err == nil {
		return version, nil
	}
	err = h.tryLoginIfCanOrSelf(err)
	if err != nil {
		return nil, err
	}
	return h.client.GetVersion()
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

func (h *ChatLineHandler) listSessions() ([]ai.Session, error) {
	sessions, err := h.client.ListSessions()
	if err == nil {
		return sessions, nil
	}
	err = h.tryLoginIfCanOrSelf(err)
	if err != nil {
		return nil, err
	}
	return h.client.ListSessions()
}

func (h *ChatLineHandler) HandleLine(line string) {
	if line == ":ls" {
		sessions, err := h.listSessions()
		if err != nil {
			fmt.Printf("List Sessions failed: %v\n", err)
			return
		}
		slices.SortFunc(sessions, func(lhs ai.Session, rhs ai.Session) int {
			return int(lhs.SessionCommon().UpdateTimeEpochMilli - rhs.SessionCommon().UpdateTimeEpochMilli)
		})
		PrintSessionTable(sessions)
		return
	}
	if line == ":version" {
		version, err := h.getVersion()
		if err != nil {
			fmt.Printf("Get Version failed: %v\n", err)
			return
		}
		fmt.Println(version)
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
				fmt.Printf("Create Session failed: %v\n", err)
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
