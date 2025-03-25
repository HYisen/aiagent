package chat

import (
	"encoding/json"
	"strings"
	"testing"
)

type Widget struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

func TestJSONEscapeLine(t *testing.T) {
	passage := `line1
line2
line3`
	w := &Widget{
		ID:      19,
		Message: passage,
	}
	data, err := json.Marshal(w)
	if err != nil {
		t.Errorf("JSON encode with not nil error: %s", err)
	}
	s := string(data)
	if strings.Contains(s, "\n") {
		t.Errorf("JSON not encode with escape LF: %s", s)
	}
	//goland:noinspection SpellCheckingInspection
	want := `{"id":19,"message":"line1\nline2\nline3"}`
	if want != s {
		t.Errorf("JSON encode result changed got %s want %s", s, want)
	}
}
