package model

import "testing"

func TestSession_WeakName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"happy path", "2025-04-07 08:06:54.421831436 -0400 EDT m=+591042.754616192", true},
		{"none", "", false},
		{"manually defined", "this_is_a_session", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				Name: tt.input,
			}
			if got := s.WeakName(); got != tt.want {
				t.Errorf("WeakName() = %v, want %v", got, tt.want)
			}
		})
	}
}
