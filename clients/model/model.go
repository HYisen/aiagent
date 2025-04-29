// Package model stores models used by gen.
// It's not mandatory; just comparing to the other choice to implement GenInternalDoName for many times,
// I would rather pull all the structs needed together here.
// See https://github.com/go-gorm/gen/issues/971
package model

import (
	"aiagent/clients/openai"
	"time"
)

type Session struct {
	ID       int `json:"-"`
	Name     string
	UserID   int
	ScopedID int
	Chats    []*Chat `gorm:"foreignkey:SessionID"`
}

func (s *Session) SessionWithID() *SessionWithID {
	return &SessionWithID{
		ID:      s.ID,
		Session: *s,
	}
}

type SessionWithID struct {
	ID int
	Session
}

func DefaultSessionName() string {
	return time.Now().String()
}

func (s *Session) History() []openai.Message {
	var ret []openai.Message
	for _, chat := range s.Chats {
		c := chat.Chat()
		if !c.Valid() {
			continue
		}
		ret = append(ret, c.HistoryRecords()...)
	}
	return ret
}

type Chat struct {
	ID         int `json:"-"`
	SessionID  int `json:"-"`
	Input      string
	CreateTime int64
	Result     *Result `gorm:"foreignkey:ChatID"`
}

func (c *Chat) Chat() *openai.Chat {
	return &openai.Chat{
		Input:   c.Input,
		Created: time.UnixMilli(c.CreateTime),
		Result:  c.Result.ChatCompletion(),
	}
}

type Result struct {
	ID                int `json:"-"`
	ChatID            int `json:"-"`
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

func NewResult(cc *openai.ChatCompletion) *Result {
	return &Result{
		ID:                   0, // leave null for generated PK
		ChatID:               0, // leave null for FK fulfilling
		ChatCompletionID:     cc.ID,
		Created:              cc.Created,
		Model:                cc.Model,
		SystemFingerprint:    cc.SystemFingerprint,
		FinishReason:         cc.Choices[0].FinishReason,
		Role:                 cc.Choices[0].Message.Role,
		Content:              cc.Choices[0].Message.Content,
		ReasoningContent:     cc.Choices[0].Message.ReasoningContent,
		PromptTokens:         cc.Usage.PromptTokens,
		CompletionTokens:     cc.Usage.CompletionTokens,
		CachedTokens:         cc.Usage.PromoteTokensDetails.CachedTokens,
		ReasoningTokens:      cc.Usage.CompletionTokensDetails.ReasoningTokens,
		PromptCacheHitTokens: cc.Usage.PromptCacheHitTokens,
	}
}

func (r *Result) ChatCompletion() *openai.ChatCompletion {
	if r == nil {
		return nil
	}
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

type User struct {
	ID               int
	Nickname         string
	SessionsSequence int
}
