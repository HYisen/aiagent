package openai

import "fmt"

type ChatModel = string // it's openai.ChatModel

// Define enums for better understanding over name, not supposed to be all used
// ref https://api-docs.deepseek.com/quick_start/pricing
//
//goland:noinspection GoUnusedConst
const (
	ChatModelDeepSeekV3 ChatModel = "deepseek-chat"
	ChatModelDeepSeekR1 ChatModel = "deepseek-reasoner"
)

type Request struct {
	Messages []Message `json:"messages"`
	Model    ChatModel `json:"model"`
}

type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

func (m Message) Print() {
	fmt.Printf("role: %s\n", m.Role)
	fmt.Println("> reason")
	fmt.Println(m.ReasoningContent)
	fmt.Println("> content")
	fmt.Println(m.Content)
}

type Response struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"` // epoch second
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerPrint string   `json:"system_finger_print"`
}

type FinishReason = string

// Define enums for better understanding over that defined in vendor documents, not supposed to be all used
//
//goland:noinspection GoUnusedConst
const (
	FinishReasonStop                       FinishReason = "stop"
	FinishReasonLength                     FinishReason = "length"
	FinishReasonContentFilter              FinishReason = "content_filter"
	FinishReasonToolCalls                  FinishReason = "tool_calls"
	FinishReasonInsufficientSystemResource FinishReason = "insufficient_system_resource"
)

type Choice struct {
	Index        int          `json:"index"`
	Message      Message      `json:"message"`
	FinishReason FinishReason `json:"finish_reason,omitempty"` // not enum as it's other decided.
	// field logprobs is ignored as it's null as long as I have not supported to require it in Request yet
}

type Usage struct {
	PromptTokens           int                    `json:"prompt_tokens"`
	CompletionTokens       int                    `json:"completion_tokens"`
	TotalTokens            int                    `json:"total_tokens"`
	PromoteTokenDetails    PromoteTokenDetails    `json:"promote_token_details"`
	CompletionTokenDetails CompletionTokenDetails `json:"completion_token_details"`
	PromptCacheHitTokens   int                    `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens  int                    `json:"prompt_cache_miss_tokens"`
}

type PromoteTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type CompletionTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}
