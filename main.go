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

	err := service.OneShot("say this is a test")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nnext stream")
	err = service.OneShotStream("say this is a test")
	if err != nil {
		log.Fatal(err)
	}
}
