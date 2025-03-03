package main

import (
	"aiagent/clients/openai"
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var DeepSeekAPIKey = flag.String("DeepSeekAPIKey", "this_is_a_secret", "API Key from platform.deepseek.com/api_keys")

var mode = flag.String("mode", "REPL", "app mode from SmokeTest|REPL|server")

func main() {
	flag.Parse()
	switch *mode {
	case "REPL":
		repl()
	case "SmokeTest":
		basic()
	default:
		log.Fatalf("unsupported mode %s", *mode)
	}
}

func repl() {
	service := openai.NewService("https://api.deepseek.com", *DeepSeekAPIKey)
	var history []openai.Message

	scanner := bufio.NewScanner(os.Stdin)
	const escapeLine = "exit()"
	fmt.Printf("hint: input %s to escape\n", escapeLine)
	for scanner.Scan() {
		line := scanner.Text()
		if line == escapeLine {
			break
		}
		log.Printf("read line : %s\n", line)
		history = append(history, openai.Message{
			Role:             "user",
			Content:          line,
			ReasoningContent: "",
		})
		fmt.Printf("sending with history size %d\n", len(history))
		cc, err := service.OneShotStream(context.Background(), openai.Request{
			Messages: history,
			Model:    openai.ChatModelDeepSeekR1,
		}, NewPrintWordChannel())
		if err != nil {
			log.Fatal(err)
		}
		history = append(history, cc.Choices[0].Message.AsHistoryRecord())
	}
}

func basic() {
	service := openai.NewService("https://api.deepseek.com", *DeepSeekAPIKey)
	req := openai.Request{
		Messages: []openai.Message{{
			Role:    "user",
			Content: "say this is a test",
		}},
		Model: openai.ChatModelDeepSeekR1,
	}

	rsp, err := service.OneShot(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", rsp)

	fmt.Println("\nnext stream")
	aggregated, err := service.OneShotStream(context.Background(), req, NewPrintItemChannel())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\naggregated:\n%+v\n", aggregated)
}

func NewPrintItemChannel() chan<- openai.ChatCompletionChunk {
	ret := make(chan openai.ChatCompletionChunk)
	go func(ch <-chan openai.ChatCompletionChunk) {
		for chunk := range ch {
			fmt.Printf("%+v\n", chunk)
		}
		log.Println("end of print channel")
	}(ret)
	return ret
}

func NewPrintWordChannel() chan<- openai.ChatCompletionChunk {
	ret := make(chan openai.ChatCompletionChunk)
	go func(ch <-chan openai.ChatCompletionChunk) {
		var count int
		var cotFinished bool
		for chunk := range ch {
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
				separation := strings.Repeat("-", 8)
				fmt.Print("\n\n" + separation + " CoT END " + separation + "\n\n")
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
