package matcher

import (
	"testing"
)

func TestMatcher(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantParseError bool
		wantPass       []int
		wantFail       []int
	}{
		{"all", "*", false, []int{0, 1, 20, 486}, []int{}},
		{"bad input", "**", true, []int{}, []int{}},
		{"bad head", "1,a-10", true, []int{}, []int{}},
		{"bad tail", "5-b,16", true, []int{}, []int{}},
		{"exacts", "1,3,5,8", false, []int{1, 3, 5, 8}, []int{0, 2, 4, 6, 7}},
		{"ranges", "1-3,6-8", false, []int{1, 2, 3, 6, 7, 8}, []int{0, 4, 9}},
		{"mixed", "20-30,44", false, []int{20, 25, 30, 44}, []int{19, 42, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Parse(tt.input)
			if (err != nil) != tt.wantParseError {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantParseError)
			}
			for _, pass := range tt.wantPass {
				if m.Match(pass) != true {
					t.Errorf("Match(%d) = %v, want %v", pass, m.Match(pass), true)
				}
			}
			for _, fail := range tt.wantFail {
				if m.Match(fail) != false {
					t.Errorf("Match(%d) = %v, want %v", fail, m.Match(fail), false)
				}
			}
		})
	}
}
