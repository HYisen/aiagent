// Package model stores models used by gen.
// It's not mandatory, just comparing to the other choice to implements GenInternalDoName for many times,
// I would rather pull all the structs needed together here.
// ref https://github.com/go-gorm/gen/issues/971
package model

import (
	"aiagent/clients/openai"
	"time"
)

type Session struct {
	ID     int
	Name   string
	UserID int
}

type Chat struct {
	ID         int
	SessionID  int
	Input      string
	CreateTime int
	Result     *Result `gorm:"foreignkey:ChatID"`
}

func (c *Chat) Chat() *openai.Chat {
	return &openai.Chat{
		Input:   c.Input,
		Created: time.UnixMilli(int64(c.CreateTime)),
		Result:  c.Result.ChatCompletion(),
	}
}

type Result struct {
	ID                int
	ChatID            int
	ChatCompletionID  string
	Created           int64
	Model             openai.ChatModel
	SystemFingerprint string
	FinishReason      string

	Role             string
	Content          string
	ReasoningContent string

	PromptTokens         int
	CompletionTokens     int
	CachedTokens         int
	ReasoningTokens      int
	PromptCacheHitTokens int
}

func (r *Result) ChatCompletion() *openai.ChatCompletion {
	return &openai.ChatCompletion{
		ChatCompletionBase: openai.ChatCompletionBase{
			ID:                r.ChatCompletionID,
			Created:           r.Created,
			Model:             r.Model,
			SystemFingerprint: r.SystemFingerprint,
		},
		Choices: []openai.Choice{{
			Index: 0,
			Message: openai.Message{
				Role:             r.Role,
				Content:          r.Content,
				ReasoningContent: r.ReasoningContent,
			},
			FinishReason: r.FinishReason,
		}},
		Usage: openai.Usage{
			PromptTokens:            r.PromptTokens,
			CompletionTokens:        r.CompletionTokens,
			TotalTokens:             r.PromptTokens + r.CompletionTokens,
			PromoteTokensDetails:    openai.PromoteTokensDetails{CachedTokens: r.CachedTokens},
			CompletionTokensDetails: openai.CompletionTokensDetails{ReasoningTokens: r.ReasoningTokens},
			PromptCacheHitTokens:    r.PromptCacheHitTokens,
			PromptCacheMissTokens:   r.PromptTokens - r.PromptCacheHitTokens,
		},
	}
}
