// Package console provides helpers for user and app interacts with console, a.k.a. TTY or terminal.
package console

import (
	"aiagent/clients/openai"
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
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

type Controller struct {
	handler LineHandler
	opts    Options
}

// NewController creates *Controller, use NewDefaultOptions to provide a workable opts or make it yourself.
func NewController(handler LineHandler, opts Options) *Controller {
	return &Controller{handler: handler, opts: opts}
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

type LineHandler interface {
	HandleLine(line string)
}

type HandleLineFunc func(line string)

func (c *Controller) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("hint: input %s to escape\n", c.opts.EscapeLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == c.opts.EscapeLine {
			break
		}
		if c.opts.EchoInput {
			log.Printf("read line : %s\n", line)
		}
		c.handler.HandleLine(line)
	}
}
