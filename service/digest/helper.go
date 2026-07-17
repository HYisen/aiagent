package digest

import (
	"aiagent/clients/model"
	"aiagent/clients/openai"
	_ "embed"
	"log/slog"
	"net/http"
	"strings"
	"text/template"

	"github.com/hyisen/wf"
)

func Digest(chats []*model.Chat, safeWord string) (prompt string, err *wf.CodedError) {
	quota := 100_000 // Limited by context length and cost. It's soft, real tokens shall be a bit more.
	var messages []Message
	for _, chat := range chats {
		if chat.Result == nil {
			slog.Warn("Digest skip chat with nil result", "chat", chat)
			continue
		}
		if chat.Result.FinishReason != openai.FinishReasonStop {
			slog.Warn("Digest skip chat abnormal finished", "finish_reason", chat.Result.FinishReason, "chat", chat)
			continue
		}
		size := len(chat.Input) + len(chat.Result.Content)
		if quota < size {
			break
		}
		quota -= size
		messages = append(messages, Message{
			Role:    "user",
			Content: chat.Input,
		}, Message{
			Role:    chat.Result.Role,
			Content: chat.Result.Content,
		})
	}

	if len(messages) == 0 {
		return "", wf.NewCodedErrorf(http.StatusRequestEntityTooLarge, "no history left under limit %d", quota)
	}

	var sb strings.Builder
	if err := promptTmpl.Execute(&sb, PromptParams{
		Messages: messages,
		SafeWord: safeWord,
	}); err != nil {
		return "", wf.NewCodedErrorf(http.StatusInternalServerError, "execute template: %v", err)
	}
	return sb.String(), nil
}

//go:embed prompt.tmpl
var templateText string
var promptTmpl = template.Must(template.New("prompt").Parse(templateText))

type Message struct {
	Role    string
	Content string
}

type PromptParams struct {
	Messages []Message
	SafeWord string
}
