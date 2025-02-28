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

// Define enums for better understanding over name, not supposed to be all used
// ref https://api-docs.deepseek.com/quick_start/pricing
//
//goland:noinspection GoUnusedConst
const (
	ChatModelDeepSeekV3 openai.ChatModel = "deepseek-chat"
	ChatModelDeepSeekR1 openai.ChatModel = "deepseek-reasoner"
)

var DeepSeekAPIKey = flag.String("DeepSeekAPIKey", "this_is_a_secret", "API Key from platform.deepseek.com/api_keys")

func main() {
	service := NewService(NewDeepSeekClient(*DeepSeekAPIKey))
	msg, err := service.OneShot("write a haiku about ai")
	if err != nil {
		log.Fatalln(err)
	}
	msg.Print()
}

type Service struct {
	client *openai.Client
}

func NewService(client *openai.Client) *Service {
	return &Service{client: client}
}

func NewDeepSeekClient(apiKey string) *openai.Client {
	return openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.deepseek.com"),
	)
}

func (s *Service) OneShot(content string) (*Message, error) {
	chatCompletion, err := s.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		}),
		Model: openai.F(ChatModelDeepSeekR1),
	})
	if err != nil {
		return nil, fmt.Errorf("fetch response: %w", err)
	}
	if len(chatCompletion.Choices) != 1 {
		return nil, fmt.Errorf("not single choices %d in resp %+v", len(chatCompletion.Choices), chatCompletion.Choices)
	}
	choice := chatCompletion.Choices[0]
	if choice.FinishReason != "stop" {
		// expected from stop, content_filter has been witnessed
		return nil, fmt.Errorf("unexpected finish reason %s", choice.FinishReason)
	}
	fmt.Println(choice.Message.JSON.Role)
	fmt.Println(choice.Message.JSON.Refusal)
	fmt.Println(choice.Message.Content)
	fmt.Println(choice.Message.JSON.RawJSON())
	fmt.Printf("%+v\n", chatCompletion.Choices)

	var msg Message
	if err := json.Unmarshal([]byte(choice.Message.JSON.RawJSON()), &msg); err != nil {
		return nil, fmt.Errorf("parse JSON in response: %w", err)
	}
	return &msg, nil
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
