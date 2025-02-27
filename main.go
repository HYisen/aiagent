package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"log"
)

const ChatModelDeepSeekV3 openai.ChatModel = "deepseek-chat"
const ChatModelDeepSeekR1 openai.ChatModel = "deepseek-reasoner"

var DeepSeekAPIKey = flag.String("DeekSeekAPIKey", "this_is_a_secret", "API Key from platform.deepseek.com/api_keys")

func main() {
	client := openai.NewClient(
		option.WithAPIKey(*DeepSeekAPIKey),
		option.WithBaseURL("https://api.deepseek.com"),
	)
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("write a haiku about ai"),
		}),
		Model: openai.F(ChatModelDeepSeekR1),
	})
	if err != nil {
		panic(err)
	}
	if len(chatCompletion.Choices) != 1 {
		log.Printf("not single choices %d in response %+v", len(chatCompletion.Choices), chatCompletion.Choices)
	}
	choice := chatCompletion.Choices[0]
	if choice.FinishReason != "stop" {
		// expected from stop, content_filter has been witnessed
		log.Printf("unexpected finish reason %s", choice.FinishReason)
	}
	fmt.Println(choice.Message.JSON.Role)
	fmt.Println(choice.Message.JSON.Refusal)
	fmt.Println(choice.Message.Content)
	fmt.Println(choice.Message.JSON.RawJSON())
	fmt.Printf("%+v\n", chatCompletion.Choices)

	var msg Message
	if err := json.Unmarshal([]byte(choice.Message.JSON.RawJSON()), &msg); err != nil {
		log.Fatalln(err)
	}
	msg.Print()
}

type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

func (m Message) Print() {
	fmt.Printf("role: %s\n", m.Role)
	fmt.Println("> reason")
	fmt.Println(m.ReasoningContent)
	fmt.Println("> content")
	fmt.Println(m.Content)
}
