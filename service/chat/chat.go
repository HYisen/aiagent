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
	Content string `json:"content"`
	Model   string `json:"model"`
}

func (s *Service) Chat(ctx context.Context, sessionID int, req *Request) (*openai.ChatCompletion, *wf.CodedError) {
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

func (s *Service) prepareChat(ctx context.Context, sessionID int, req *Request) (
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

func (s *Service) ChatStream(ctx context.Context, sessionID int, req *Request) (<-chan wf.MessageEvent, *wf.CodedError) {
	ses, neo, e := s.prepareChat(ctx, sessionID, req)
	if e != nil {
		return nil, e
	}

	up, err := s.client.OneShotStreamFast(ctx, openai.Request{
		Messages: append(ses.History(), openai.NewUserMessage(req.Content)),
		Model:    req.Model,
	})
	if err != nil {
		return nil, wf.NewCodedErrorf(http.StatusInternalServerError, "upstream: %v", err.Error())
	}

	down := make(chan wf.MessageEvent)
	go s.translateAggregateSave(ctx, up, down, neo)
	return down, nil
}

func (s *Service) translateAggregateSave(
	ctx context.Context,
	up <-chan openai.ChatCompletionChunk,
	down chan<- wf.MessageEvent,
	neo *model.Chat,
) {
	defer close(down)
	aggregator := openai.NewAggregator()
	var stage int

	for {
		select {
		case <-ctx.Done():
			slog.Warn("stream interrupted", "error", ctx.Err())
			return
		case chunk, ok := <-up:
			if !ok {
				if stage != 3 {
					slog.Error("end with unexpected status", "stage", stage)
				}
				neo.Result = model.NewResult(aggregator)
				if err := s.chatRepository.Save(ctx, neo); err != nil {
					slog.Error("can not append record in stream mode", "chat", neo)
					down <- NewErrorMessageEvent(err)
				}
				return
			}
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
		// One single LF would become 2 data: with empty value, build to 1 LF again in client.
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
// If fails, log err and NewErrorMessageEvent invoked with err would be returned.
func NewJSONMessageEvent(typeOptional string, item any) wf.MessageEvent {
	data, err := json.Marshal(item)
	if err != nil {
		slog.Error("NewJSONMessageEvent encode", "err", err, "item", item)
		return NewErrorMessageEvent(fmt.Errorf("parse item %+v to JSON: %w", item, err))
	}
	return wf.MessageEvent{
		TypeOptional: typeOptional,
		// JSON marshaller escape LF, TestJSONEscapeLine assert that. No split by line needed here.
		Lines: []string{string(data)},
	}
}
