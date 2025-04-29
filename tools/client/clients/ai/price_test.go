package ai

import (
	"golang.org/x/text/currency"
	"testing"
)

type Stat [3]int

func (s Stat) InputTokens() int {
	return s[0]
}

func (s Stat) CachedInputTokens() int {
	return s[1]
}

func (s Stat) OutputTokens() int {
	return s[2]
}

func TestPriceMillPerMToken_Cost(t *testing.T) {
	tests := []struct {
		name string
		p    PriceMillPerMToken
		s    TokenUsageStat
		want string
	}{
		{"happy path", PriceMillPerMToken{
			Input:       1_000_000,
			CachedInput: 1_000,
			Output:      1_000_000_000,
			Unit:        currency.XAG,
		}, Stat([3]int{1, 2, 3}), "3001.002 XAG"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Cost(tt.s); got != tt.want {
				t.Errorf("Cost() = %v, want %v", got, tt.want)
			}
		})
	}
}
