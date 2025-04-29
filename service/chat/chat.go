package chat

import (
	"aiagent/clients/chat"
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyisen/wf"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	client            *openai.Client
	chatRepository    *chat.Repository
	sessionRepository *session.Repository
}

func NewService(
	client *openai.Client,
	chatRepository *chat.Repository,
	sessionRepository *session.Repository,
) *Service {
	return &Service{client: client, chatRepository: chatRepository, sessionRepository: sessionRepository}
}

type Request struct {
	UserID          int
	SessionScopedID int
	RequestPayload
}

type RequestPayload struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

func (s *Service) Chat(
	ctx context.Context,
	req *Request,
) (*openai.ChatCompletion, *wf.CodedError) {
	sessionID, err := s.sessionRepository.FindIDByUserIDAndScopedID(ctx, req.UserID, req.SessionScopedID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session %d-%d to chat", req.UserID, req.SessionScopedID)
	}
	return s.ChatSimple(ctx, sessionID, &req.RequestPayload)
}

func (s *Service) ChatSimple(
	ctx context.Context,
	sessionID int,
	req *RequestPayload,
) (*openai.ChatCompletion, *wf.CodedError) {
	ses, neo, e := s.prepareChat(ctx, sessionID, req)
	if e != nil {
		return nil, e
	}

	chatCompletion, err := s.client.OneShot(ctx, openai.Request{
		Messages: append(ses.History(), openai.NewUserMessage(req.Content)),
		Model:    req.Model,
	})
	if err != nil {
		return nil, wf.NewCodedErrorf(http.StatusInternalServerError, "upstream: %v", err.Error())
	}
	neo.Result = model.NewResult(chatCompletion)

	if err := s.chatRepository.Save(ctx, neo); err != nil {
		slog.Error("can not append record", "chat", neo)
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return chatCompletion, nil
}

func (s *Service) prepareChat(ctx context.Context, sessionID int, req *RequestPayload) (
	ses *model.Session,
	neo *model.Chat,
	err *wf.CodedError,
) {
	ses, e := s.sessionRepository.FindWithChats(ctx, sessionID)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v to chat", sessionID)
	}
	if e != nil {
		return nil, nil, wf.NewCodedError(http.StatusInternalServerError, e)
	}

	if err := s.chatRepository.Save(ctx, &model.Chat{
		ID:         0,
		SessionID:  ses.ID,
		Input:      req.Content,
		CreateTime: time.Now().UnixMilli(),
		Result:     nil,
	}); err != nil {
		return nil, nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	neo, e = s.chatRepository.FindLastBySessionID(ctx, ses.ID)
	if e != nil {
		return nil, nil, wf.NewCodedError(http.StatusInternalServerError, e)
	}

	return ses, neo, nil
}

func (s *Service) ChatStream(ctx context.Context, req *Request) (<-chan wf.MessageEvent, *wf.CodedError) {
	sessionID, err := s.sessionRepository.FindIDByUserIDAndScopedID(ctx, req.UserID, req.SessionScopedID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(
			http.StatusNotFound,
			"no session %d-%d to chat stream",
			req.UserID,
			req.SessionScopedID,
		)
	}
	return s.ChatStreamSimple(ctx, sessionID, &req.RequestPayload)
}

func detachedContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx := context.WithoutCancel(parent)
	if deadline, ok := parent.Deadline(); ok {
		return context.WithDeadline(ctx, deadline)
	} else {
		return context.WithCancel(ctx)
	}
}

func (s *Service) ChatStreamSimple(
	ctx context.Context,
	sessionID int,
	req *RequestPayload,
) (<-chan wf.MessageEvent, *wf.CodedError) {
	ses, neo, e := s.prepareChat(ctx, sessionID, req)
	if e != nil {
		return nil, e
	}

	detachedCtx, detachedCancelFunc := detachedContext(ctx)
	// If we use ctx here, once the client has gone, our chat to upstream would be forced to end, which is not ideal.
	up, err := s.client.OneShotStreamFast(detachedCtx, openai.Request{
		Messages: append(ses.History(), openai.NewUserMessage(req.Content)),
		Model:    req.Model,
	})
	if err != nil {
		detachedCancelFunc()
		return nil, wf.NewCodedErrorf(http.StatusInternalServerError, "upstream: %v", err.Error())
	}

	down := make(chan wf.MessageEvent)
	// If ctx done, detachedCtx would go on the record procedure.
	go s.translateAggregateSave(detachedCtx, detachedCancelFunc, ctx, up, down, neo)
	return down, nil
}

func (s *Service) drainStreamAndRecordValidResult(
	ctx context.Context,
	up <-chan openai.ChatCompletionChunkOrError,
	down chan<- wf.MessageEvent,
	neo *model.Chat,
	aggregator *openai.ChatCompletion,
) {
	// In the happy path, up should have closed gracefully, but in the client-gone situation,
	// the up should have kept going, and we shall drain all the remained out here.
	var drainCount int
	for chunk := range up {
		drainCount++
		aggregator.Aggregate(chunk.ChatCompletionChunk)
	}

	if aggregator.Valid() {
		neo.Result = model.NewResult(aggregator)
		if err := s.chatRepository.Save(ctx, neo); err != nil {
			slog.Error("can not append record in stream mode", "chat", neo, "err", err)
			// If up can be drained, most likely the client has gone, and down is not writeable.
			if drainCount == 0 {
				down <- NewErrorMessageEvent(err)
			}
		}
	}

	if drainCount > 0 {
		slog.Info("client has gone but the result is saved", "drained", drainCount)
	}
}

func (s *Service) translateAggregateSave(
	ctx context.Context,
	cancelFunc context.CancelFunc,
	clientGone context.Context,
	up <-chan openai.ChatCompletionChunkOrError,
	down chan<- wf.MessageEvent,
	neo *model.Chat,
) {
	defer close(down)
	defer cancelFunc()
	aggregator := openai.NewAggregator()
	defer s.drainStreamAndRecordValidResult(ctx, up, down, neo, aggregator)
	var stage int

	for {
		select {
		case <-ctx.Done():
			slog.Warn("stream interrupted", "error", ctx.Err())
			return
		case <-clientGone.Done():
			slog.Warn("client gone", "error", clientGone.Err())
			return
		case coe, ok := <-up:
			if !ok {
				if stage != 3 {
					slog.Error("end with unexpected status", "stage", stage)
				}
				return
			}
			if coe.Error != nil {
				down <- NewErrorMessageEvent(coe.Error)
				continue
			}
			chunk := coe.ChatCompletionChunk
			aggregator.Aggregate(chunk)
			switch stage {
			case 0:
				stage++
				down <- NewJSONMessageEvent("head", chunk.ChatCompletionBase)
				down <- wf.MessageEvent{
					TypeOptional: "role",
					Lines:        []string{chunk.Choices[0].Delta.Role},
				}
			case 1:
				if chunk.Choices[0].Delta.Content == "" {
					down <- NewMultiLineMessageEvent(chunk.Choices[0].Delta.ReasoningContent)
				} else {
					stage++
					down <- wf.MessageEvent{
						TypeOptional: "cotEnd",
						Lines:        nil,
					}
					down <- NewMultiLineMessageEvent(chunk.Choices[0].Delta.Content)
				}
			case 2:
				if chunk.Usage == nil {
					down <- NewMultiLineMessageEvent(chunk.Choices[0].Delta.Content)
				} else {
					stage++
					down <- wf.MessageEvent{
						TypeOptional: "finish",
						Lines:        []string{*chunk.Choices[0].FinishReason},
					}
					down <- NewJSONMessageEvent("usage", chunk.Usage)
				}
			}
		}
	}
}
func NewMultiLineMessageEvent(passage string) wf.MessageEvent {
	return wf.MessageEvent{
		TypeOptional: "",
		// One single LF would become 2 data: with empty value, build to 1 LF again in clients.
		Lines: strings.Split(passage, "\n"),
	}
}

func NewErrorMessageEvent(e error) wf.MessageEvent {
	return wf.MessageEvent{
		TypeOptional: "error",
		Lines:        strings.Split(e.Error(), "\n"),
	}
}

// NewJSONMessageEvent marshall item to JSON string, put alone with typeOptional to the returned value.
// If fails, it logs and returns one generated by [NewErrorMessageEvent].
func NewJSONMessageEvent(typeOptional string, item any) wf.MessageEvent {
	data, err := json.Marshal(item)
	if err != nil {
		slog.Error("NewJSONMessageEvent encode", "err", err, "item", item)
		return NewErrorMessageEvent(fmt.Errorf("parse item %+v to JSON: %w", item, err))
	}
	return wf.MessageEvent{
		TypeOptional: typeOptional,
		// JSON marshaler escapes LF, TestJSONEscapeLine assert that.
		// So we don't need a strings.Split(string(data), "\n") to avoid LF in data.
		Lines: []string{string(data)},
	}
}
