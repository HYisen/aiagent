package openai

import "time"

// Chat is a combination of question and its possible answer.
type Chat struct {
	Input   string          `json:"input"`
	Created time.Time       `json:"created"`
	Result  *ChatCompletion `json:"result"`
}

func (c Chat) Valid() bool {
	return c.Result != nil && len(c.Result.Choices) == 1 && c.Result.Choices[0].FinishReason == FinishReasonStop
}

func (c Chat) HistoryRecords() []Message {
	var ret []Message
	ret = append(ret, NewUserMessage(c.Input))
	ret = append(ret, c.Result.Choices[0].Message.HistoryRecord())
	return ret
}
