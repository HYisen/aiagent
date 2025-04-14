package ai

import (
	"aiagent/clients/openai"
	"fmt"
	"golang.org/x/text/currency"
)

type PriceMillPerMToken struct {
	Input       int
	CachedInput int
	Output      int
	Unit        currency.Unit
}

func NewDeepSeekReasonerPrice() PriceMillPerMToken {
	// https://api-docs.deepseek.com/zh-cn/quick_start/pricing/
	// snapshot of deepseek-reasoner from Apr.14th 2025
	return PriceMillPerMToken{
		Input:       4_000,
		CachedInput: 1_000,
		Output:      16_000,
		Unit:        currency.CNY,
	}
}

func (p PriceMillPerMToken) Cost(s TokenUsageStat) string {
	var ppb int // Parts Per Billion = mill unit per Million
	ppb += p.Input * s.InputTokens()
	ppb += p.CachedInput * s.CachedInputTokens()
	ppb += p.Output * s.OutputTokens()
	return fmt.Sprintf("%.3f %s", float64(ppb)/1_000_000_100, p.Unit.String())
}

type TokenUsageStat interface {
	InputTokens() int
	CachedInputTokens() int
	OutputTokens() int
}

type OpenAIUsage openai.Usage

func (u OpenAIUsage) InputTokens() int {
	return u.PromptTokens - u.PromoteTokensDetails.CachedTokens
}

func (u OpenAIUsage) CachedInputTokens() int {
	return u.PromoteTokensDetails.CachedTokens
}

func (u OpenAIUsage) OutputTokens() int {
	return u.CompletionTokens
}
