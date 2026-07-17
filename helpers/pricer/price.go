package pricer

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

// PriceOrDefault gets input model's price, get zero(free) cost if unmatch.
//
// One may consider making [PriceMillPerMToken.Cost] a method of [ChatModel] to accomplish this polymorphism.
// But since price don't belong to package where [openai.ChatModel] exists, pulling out the enum would create
// dependency loop between the old and new package [ChatModel] exists.
// As [OpenAIUsage] and [openai.Request] would require each other's package.
// One solution is preventing package openai from using [ChatModel] directly,
// but using String or other interface instead. I introduced enum to help developers finding ChatModel,
// another layer is not acceptable, thus I leave the cost as an extension of model here.
// At present, I hold the idea that CostManager.Find(model) is better than model.Cost().
func PriceOrDefault(model openai.ChatModel) PriceMillPerMToken {
	// https://api-docs.deepseek.com/zh-cn/quick_start/pricing/
	// snapshot of deepseek-reasoner from May.6th 2026
	switch model {
	case openai.ChatModelDeepSeekV4Flash:
		return PriceMillPerMToken{
			Input:       1_000,
			CachedInput: 20,
			Output:      2_000,
			Unit:        currency.CNY,
		}
	case openai.ChatModelDeepSeekV4Pro:
		return PriceMillPerMToken{
			Input:       3_000,
			CachedInput: 25,
			Output:      6_000,
			Unit:        currency.CNY,
		}
	default:
		return PriceMillPerMToken{
			Input:       0,
			CachedInput: 0,
			Output:      0,
			Unit:        currency.XXX,
		}
	}
}

func (p PriceMillPerMToken) Cost(s TokenUsageStat) string {
	var ppb int // Parts Per Billion = mill unit per Million
	ppb += p.Input * s.InputTokens()
	ppb += p.CachedInput * s.CachedInputTokens()
	ppb += p.Output * s.OutputTokens()
	return fmt.Sprintf("%.3f %s", float64(ppb)/1_000_000_000, p.Unit.String())
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
