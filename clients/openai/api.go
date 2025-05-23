package openai

import (
	"errors"
	"fmt"
)

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

type RequestWhole struct {
	Request
	Stream bool `json:"stream"`
}

type Message struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// HistoryRecord on an item in [Response] converts it to that in [Request].
// At present, it drops the CoT field as the requirement from
// https://api-docs.deepseek.com/guides/reasoning_model#multi-round-conversation
func (m Message) HistoryRecord() Message {
	return Message{
		Role:             m.Role,
		Content:          m.Content,
		ReasoningContent: "",
	}
}

func (m Message) Print() {
	fmt.Printf("role: %s\n", m.Role)
	fmt.Println("> reason")
	fmt.Println(m.ReasoningContent)
	fmt.Println("> content")
	fmt.Println(m.Content)
}

type Response struct {
	Object string `json:"object"`
	ChatCompletion
}

type ChunkResponse struct {
	Object string `json:"object"`
	ChatCompletionChunk
}

type ChatCompletion struct {
	ChatCompletionBase
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

func (cc *ChatCompletion) Valid() bool {
	// Zero value is not valid. But NewAggregator does not create an empty one.
	// According to observed stream behaviour, Role is the first field to be fulfilled,
	// whose empty indicates a no data result.
	return len(cc.Choices) > 0 && cc.Choices[0].Message.Role != ""
}

// NewAggregator creates an *ChatCompletion for aggregation of ChatCompletionChunk.
// Itself would become a complete ChatCompletion once every ChatCompletionChunk in stream mode is aggregated.
func NewAggregator() *ChatCompletion {
	// Initiate choices with len 1 as Aggregate does not create, don't ask me how I find it vital.
	ret := &ChatCompletion{Choices: make([]Choice, 1)}
	return ret
}

func (cc *ChatCompletion) Aggregate(chunk ChatCompletionChunk) {
	// As I observed, ChatCompletionBase among chunks always exists and are identical;
	// therefore, only copy once on empty and use one filed as the canary.
	if cc.ID == "" {
		cc.ChatCompletionBase = chunk.ChatCompletionBase
	}
	// only one not nil in the final chunk
	if chunk.Usage != nil {
		cc.Usage = *chunk.Usage
	}

	if len(chunk.Choices) != 1 || len(cc.Choices) != 1 {
		// I have never seen such kind of responses, just not supported yet.
		// Won't happen unless the source code here is implemented wrong.
		panic(fmt.Errorf(
			"aggregate ChatCompletion with not single choices %d %d: %w",
			len(cc.Choices),
			len(chunk.Choices),
			errors.ErrUnsupported,
		))
	}
	neo := chunk.Choices[0]
	// only one not nil in the final chunk
	if neo.FinishReason != nil {
		cc.Choices[0].FinishReason = *neo.FinishReason
	}
	// Ignore field Index because all of them are zero, as long as previous len(choices) assert passed.
	// It is safe to join because null strings in Message JSON are decoded to "".
	cc.Choices[0].Message.Role += neo.Delta.Role
	cc.Choices[0].Message.Content += neo.Delta.Content
	cc.Choices[0].Message.ReasoningContent += neo.Delta.ReasoningContent
}

type ChatCompletionBase struct {
	ID                string `json:"id"`
	Created           int64  `json:"created"` // epoch second
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
}

type ChatCompletionChunk struct {
	ChatCompletionBase
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage"`
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
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	// field logprobs is ignored as it's null as long as I have not supported requiring it in Request yet
}

type ChunkChoice struct {
	Index        int           `json:"index"`
	Delta        Message       `json:"delta"`
	FinishReason *FinishReason `json:"finish_reason,omitempty"`
	// field logprobs is ignored as it's null as long as I have not supported requiring it in Request yet
}

type Usage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	PromoteTokensDetails    PromoteTokensDetails    `json:"prompt_tokens_details"`
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details"`
	PromptCacheHitTokens    int                     `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens   int                     `json:"prompt_cache_miss_tokens"`
}

type PromoteTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type CompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}
