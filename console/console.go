// Package console provides helpers for user and app interacts with console, a.k.a. TTY or terminal.
package console

import (
	"aiagent/clients/openai"
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func NewPrintItemChannel() chan<- openai.ChatCompletionChunkOrError {
	ret := make(chan openai.ChatCompletionChunkOrError)
	go func(ch <-chan openai.ChatCompletionChunkOrError) {
		for coe := range ch {
			if coe.Error != nil {
				fmt.Printf("err: %v\n", coe.Error)
				continue
			}
			fmt.Printf("%+v\n", coe.ChatCompletionChunk)
		}
		log.Println("end of print channel")
	}(ret)
	return ret
}

func NewPrintWordChannel() chan<- openai.ChatCompletionChunkOrError {
	ret := make(chan openai.ChatCompletionChunkOrError)
	go func(ch <-chan openai.ChatCompletionChunkOrError) {
		var count int
		var cotFinished bool
		for chunk := range ch {
			if chunk.Error != nil {
				fmt.Printf("err: %+v\n", chunk.Error)
				continue
			}

			count++
			if count == 1 {
				deltaT := time.Now().Unix() - chunk.Created
				msg := "created at T0 on server, now on client it's T"
				if deltaT > 0 {
					msg += "+"
				} else {
					msg += "-"
				}
				msg += strconv.FormatInt(deltaT, 10)
				msg += "s"
				log.Println(msg)

				fmt.Printf("%+v\n", chunk.ChatCompletionBase)
				continue
			}

			delta := chunk.Choices[0].Delta
			if delta.Role != "" {
				fmt.Printf("role: %s\n", delta.Role)
			}
			if delta.ReasoningContent != "" {
				fmt.Print(delta.ReasoningContent)
			}
			if !cotFinished && delta.ReasoningContent == "" && delta.Content != "" {
				cotFinished = true
				fmt.Print(COTEndMessage())
			}
			if delta.Content != "" {
				fmt.Print(delta.Content)
			}

			if chunk.Usage != nil {
				fmt.Println()
				fmt.Printf("FinishReason: %s\n", *chunk.Choices[0].FinishReason)
				fmt.Printf("Usage: %+v\n", *chunk.Usage)
				continue
			}
		}
		log.Printf("end of PrintWordChannel, total useful trunk %d\n", count)
	}(ret)
	return ret
}

func COTEndMessage() string {
	separation := strings.Repeat("-", 8)
	return "\n\n" + separation + " CoT END " + separation + "\n\n"
}

// MultiLineChecker limits [console.MultiLineRemote] to use.
// See [console.MultiLineRemote] docs for methods' definition.
type MultiLineChecker interface {
	OnMultiLineEndExit(line string) bool
	MultiLine() bool
}

type Controller struct {
	handler InputHandler
	opts    Options
	remote  MultiLineChecker

	muRunning *sync.Mutex // Guard [Controller.Run] is not concurrent.
}

// NewController creates *[Controller], use [NewDefaultOptions] to provide a workable opts or make it yourself.
func NewController(handler InputHandler, opts Options, remote MultiLineChecker) *Controller {
	return &Controller{
		handler:   handler,
		opts:      opts,
		remote:    remote,
		muRunning: &sync.Mutex{},
	}
}

type Options struct {
	EscapeLine string
	EchoInput  bool
}

func NewDefaultOptions() Options {
	return Options{
		EscapeLine: "exit()",
		EchoInput:  true,
	}
}

type InputHandler interface {
	HandleInput(content string)
}

func (c *Controller) Run() {
	if !c.muRunning.TryLock() {
		panic("concurrent on Controller.Run is not supported yet")
	}
	defer func() {
		c.muRunning.Unlock()
	}()

	// Another approach I have tried but denied is making buf receiver's fields.
	// So that we can pull procedure to receiver's private method easier.
	// But then muRunning would have to be abused to protect those fields.
	// Keeping them as local variables makes it much easier to declare thread-safe.
	var buf []string

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("hint: input %s to escape\n", c.opts.EscapeLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == c.opts.EscapeLine {
			// If there is an exact c.opts.EscapeLine in multi-line input.
			// I hold further improvement until real user complain comes out.
			// Maybe user would even like current design as a stable exit.
			if len(buf) > 0 {
				slog.Warn("unhandled multi-line", "buf", buf)
			}
			break
		}
		if c.opts.EchoInput {
			log.Printf("read line : %s\n", line)
		}

		if !c.remote.MultiLine() {
			c.handler.HandleInput(line)
			continue
		}
		if c.remote.OnMultiLineEndExit(line) {
			c.handler.HandleInput(strings.Join(buf, "\n"))
			buf = nil
			continue
		}
		buf = append(buf, line)
	}
}
