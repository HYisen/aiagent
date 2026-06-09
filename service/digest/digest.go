package digest

import (
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"aiagent/helpers/matcher"
	"aiagent/helpers/pricer"
	"aiagent/helpers/runner"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
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

func (s *Service) generateTitleAndSave(ctx context.Context, sessionID int) (neo *model.Session, e *wf.CodedError) {
	ses, err := s.sessionRepository.FindWithChats(ctx, sessionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %d to digest", sessionID)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	safeWord := "I_DO_NOT_ANSWER_IT"
	prompt, ce := Digest(ses.Chats, safeWord)
	if ce != nil {
		return nil, ce
	}

	req := openai.NewRequest(
		[]openai.Message{openai.NewUserMessage(prompt)},
		s.digestChatModel(),
		openai.ReasoningEffortNone,
	)
	// `s.client.OneShotStream` behaves identical on rejected inputs,
	// as just rejected in content, with a normal FinishReasonStop.
	// Make sense as it's reject to response,
	// not response but filtered as FinishReasonContentFilter.
	// And that's why I add safe-word mechanism.
	cc, err := s.client.OneShot(ctx, req)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusServiceUnavailable, err)
	}
	name, ce := extractNewName(cc, safeWord)
	if ce != nil {
		return nil, ce
	}

	price := pricer.PriceOrDefault(s.digestChatModel()).Cost(pricer.OpenAIUsage(cc.Usage))
	slog.Info("session name generated", "name", name, "prompt_length", len(prompt), "price", price, "usage", cc.Usage)

	if err := s.sessionRepository.UpdateName(ctx, sessionID, name); err != nil {
		return nil, wf.NewCodedError(http.StatusServiceUnavailable, err)
	}
	ses.Name = name
	return ses, nil
}

// GenerateTitle does that in [ service.V1Service ] style.
func (s *Service) GenerateTitle(ctx context.Context, sessionID int) *wf.CodedError {
	_, err := s.generateTitleAndSave(ctx, sessionID)
	return err
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

func (s *Service) digestChatModel() openai.ChatModel {
	return openai.ChatModelDeepSeekV4Flash
}

func (s *Service) concurrentLimit() int {
	upstream := s.digestChatModel().ConcurrentLimit() / 2 // yield half to others
	local := runtime.NumCPU() * 20                        // fan out 20x as it's more concurrent than parallelism
	return min(upstream, local)
}

// GenerateSessionName does that in [ service.V2Service ] style.
func (s *Service) GenerateSessionName(
	ctx context.Context,
	userID int,
	scopedIDRange string,
) (scopedIDToNeoName map[int]string, e *wf.CodedError) {
	sessions, err := s.sessionRepository.FindByUserID(ctx, userID)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusServiceUnavailable, err)
	}

	mat, err := matcher.Parse(scopedIDRange)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusBadRequest, err)
	}

	var ids []int
	for _, ses := range sessions {
		if ses.WeakName() && mat.Match(ses.ScopedID) {
			ids = append(ids, ses.ID)
		}
	}

	handler := func(ctx context.Context, input int) (*model.Session, error) {
		// `s.generateTitleAndSave(ctx, input)` fails.
		// DON'T ASK ME WHY I KNOW IT!
		// (*wf.CodedError)(nil) != nil
		// The cast up is mandatory.
		neo, typedErr := s.generateTitleAndSave(ctx, input)
		if typedErr != nil {
			return nil, typedErr
		}
		return neo, nil
	}
	// Limited by timeout and upstream concurrency limit, whatever nThreads is,
	// once the matched sessions goes too many, timeout inevitably comes true.
	// We gurantee first batch complete first, so that even later got timeout,
	// users can achieve their goal step by step, in the way of manual retrys.
	results, err := runner.Run(ctx, s.concurrentLimit(), handler, ids)
	if err != nil {
		return nil, err.(*wf.CodedError) // Yes, Run return the same exact error type from handler.
	}

	scopedIDToNeoName = make(map[int]string)
	for _, result := range results {
		scopedIDToNeoName[result.ScopedID] = result.Name
	}
	return scopedIDToNeoName, nil
}
