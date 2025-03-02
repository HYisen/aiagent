package main

import (
	"aiagent/clients/openai"
	"flag"
	"fmt"
	"log"
)

var DeepSeekAPIKey = flag.String("DeepSeekAPIKey", "this_is_a_secret", "API Key from platform.deepseek.com/api_keys")

func main() {
	service := openai.NewService("https://api.deepseek.com", *DeepSeekAPIKey)
	req := openai.Request{
		Messages: []openai.Message{{
			Role:    "user",
			Content: "say this is a test",
		}},
		Model: openai.ChatModelDeepSeekR1,
	}

	rsp, err := service.OneShot(req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", rsp)

	fmt.Println("\nnext stream")
	aggregated, err := service.OneShotStream(req, NewPrintChannel())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\naggregated:\n%+v\n", aggregated)
}

func NewPrintChannel() chan<- openai.ChatCompletionChunk {
	ret := make(chan openai.ChatCompletionChunk)
	go func(ch <-chan openai.ChatCompletionChunk) {
		for chunk := range ch {
			fmt.Printf("%+v\n", chunk)
		}
		log.Println("end of print channel")
	}(ret)
	return ret
}
