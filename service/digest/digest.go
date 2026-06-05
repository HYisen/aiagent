package digest

import (
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hyisen/wf"
	"gorm.io/gorm"
)

type Service struct {
	client            *openai.Client
	sessionRepository *session.Repository
}

func NewService(client *openai.Client, sessionRepository *session.Repository) *Service {
	return &Service{client: client, sessionRepository: sessionRepository}
}

func (s *Service) GenerateTitle(ctx context.Context, sessionID int) *wf.CodedError {
	ses, err := s.sessionRepository.FindWithChats(ctx, sessionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return wf.NewCodedErrorf(http.StatusNotFound, "no session on id %d to digest", sessionID)
	}
	if err != nil {
		return wf.NewCodedError(http.StatusInternalServerError, err)
	}

	safeWord := "I_DO_NOT_ANSWER_IT"
	prompt, ce := Digest(ses.Chats, safeWord)
	if ce != nil {
		return ce
	}

	req := openai.NewRequest(
		[]openai.Message{openai.NewUserMessage(prompt)},
		openai.ChatModelDeepSeekV4Flash,
		openai.ReasoningEffortNone,
	)
	// `s.client.OneShotStream` behaves identical on rejected inputs,
	// as just rejected in content, with a normal FinishReasonStop.
	// Make sense as it's reject to response,
	// not response but filtered as FinishReasonContentFilter.
	// And that's why I add safe-word mechanism.
	cc, err := s.client.OneShot(ctx, req)
	if err != nil {
		return wf.NewCodedError(http.StatusServiceUnavailable, err)
	}
	name, ce := extractNewName(cc, safeWord)
	if ce != nil {
		return ce
	}

	slog.Info("session name generated", "name", name, "prompt_length", len(prompt), "cost", cc.Usage)

	if err := s.sessionRepository.UpdateName(ctx, sessionID, name); err != nil {
		return wf.NewCodedError(http.StatusServiceUnavailable, err)
	}
	return nil
}

func extractNewName(cc *openai.ChatCompletion, safeWord string) (string, *wf.CodedError) {
	if !cc.Valid() {
		return "", wf.NewCodedErrorf(http.StatusServiceUnavailable, "invalid %+v", cc)
	}

	if len(cc.Choices) != 1 {
		panic(fmt.Errorf("unsupported choices count %d", len(cc.Choices)))
	}
	choice := cc.Choices[0]

	switch choice.FinishReason {
	case openai.FinishReasonStop:
		content := choice.Message.Content
		if strings.Contains(content, safeWord) {
			return "", wf.NewCodedError(http.StatusUnavailableForLegalReasons, errors.New("upstream said can't"))
		}
		return content, nil
	// The following cases are just best-effort, I have never witnessed nor tested them in real.
	// Typical rejected inputs don't trigger them, just rejected in content with a normal FinishReasonStop.
	case openai.FinishReasonContentFilter:
		return "", wf.NewCodedErrorf(http.StatusUnavailableForLegalReasons, "upstream said %v", choice.FinishReason)
	case openai.FinishReasonInsufficientSystemResource:
		return "", wf.NewCodedErrorf(http.StatusServiceUnavailable, "upstream says %v", choice.FinishReason)
	case openai.FinishReasonLength:
		fallthrough
	case openai.FinishReasonToolCalls:
		fallthrough
	default:
		panic(fmt.Errorf("unexpected finish reason %q", choice.FinishReason))
	}
}
