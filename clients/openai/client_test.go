package openai

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSSEDataLineError(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		want    Error
	}{
		{
			"captured",
			`{"error":{"message":"Content Exists Risk","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`,
			Error{Inner: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Param   string `json:"param"`
				Code    string `json:"code"`
			}{
				Message: "Content Exists Risk",
				Type:    "invalid_request_error",
				Param:   "",
				Code:    "invalid_request_error",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Error
			if err := json.Unmarshal([]byte(tt.jsonStr), &got); err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode got %v, want %v", got, tt.want)
			}
			// Don't run Ouroboros, as the param null can't be understood with more examples.
		})
	}
}
