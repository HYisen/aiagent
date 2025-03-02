package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Service struct {
	baseURL string
	apiKey  string
}

func NewService(baseURL, apiKey string) *Service {
	return &Service{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (s *Service) OneShot(content string) error {
	data, err := json.Marshal(Request{
		Messages: []Message{{
			Role:    "user",
			Content: content,
		}},
		Model: ChatModelDeepSeekR1,
	})
	if err != nil {
		return err
	}

	url := s.baseURL + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+s.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("warn: potential resource leak as failed to close body: %v", err)
		}
	}(resp.Body)

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	fmt.Printf("%+v\n", response)
	return nil
}
