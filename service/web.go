package service

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
	"reflect"
	"strings"
	"time"
)

type Service struct {
	client *openai.Client
	web    *wf.Web

	sessionRepository *session.Repository
	chatRepository    *chat.Repository
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.web.ServeHTTP(writer, request)
}

func (s *Service) CreateSession(ctx context.Context) (int, *wf.CodedError) {
	name := time.Now().String()
	if err := s.sessionRepository.Save(ctx, model.Session{
		ID:   0,
		Name: name,
	}); err != nil {
		return 0, nil
	}
	id, err := s.sessionRepository.FindLastIDByUserIDAndName(ctx, 0, name)
	if err != nil {
		return 0, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return id, nil
}

func (s *Service) FindSessions(ctx context.Context) ([]*model.Session, *wf.CodedError) {
	ret, err := s.sessionRepository.FindAll(ctx)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

func (s *Service) FindSessionByID(ctx context.Context, id int) (*model.Session, *wf.CodedError) {
	ret, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v", id)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

type ChatRequest struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

func (s *Service) Chat(ctx context.Context, id int, req *ChatRequest) (*openai.ChatCompletion, *wf.CodedError) {
	ses, neo, e := s.prepareChat(ctx, id, req)
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

func (s *Service) prepareChat(ctx context.Context, id int, req *ChatRequest) (
	ses *model.Session,
	neo *model.Chat,
	err *wf.CodedError,
) {
	ses, e := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v to chat", id)
	}
	if e != nil {
		return nil, nil, wf.NewCodedError(http.StatusInternalServerError, err)
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
		return nil, nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	return ses, neo, nil
}

func (s *Service) ChatStream(ctx context.Context, id int, req *ChatRequest) (<-chan wf.MessageEvent, *wf.CodedError) {
	ses, neo, e := s.prepareChat(ctx, id, req)
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
	var stage int // 0head 1ReasoningContent 2Content 3tail
	for chunk := range up {
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
	if stage != 3 {
		slog.Error("end with unexpected status", "stage", stage)
	}
	neo.Result = model.NewResult(aggregator)
	if err := s.chatRepository.Save(ctx, neo); err != nil {
		slog.Error("can not append record in stream mode", "chat", neo)
		down <- NewErrorMessageEvent(err)
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

func New(
	client *openai.Client,
	sessionRepository *session.Repository,
	chatRepository *chat.Repository,
) *Service {
	ret := &Service{
		client:            client,
		web:               nil,
		sessionRepository: sessionRepository,
		chatRepository:    chatRepository,
	}

	v1PostSession := wf.NewJSONHandler(
		wf.Exact(http.MethodPost, "/v1/sessions"),
		reflect.TypeOf(wf.Empty{}),
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.CreateSession(ctx)
		},
	)

	v1GetSessions := wf.NewJSONHandler(
		wf.Exact(http.MethodGet, "/v1/sessions"),
		reflect.TypeOf(wf.Empty{}),
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.FindSessions(ctx)
		},
	)

	v1GetSessionByID := wf.NewClosureHandler(
		wf.ResourceWithID(http.MethodGet, "/v1/sessions/", ""),
		wf.PathIDParser(""),
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.FindSessionByID(ctx, req.(int))
		},
		json.Marshal,
		wf.JSONContentType,
	)

	v1PostSessionChatPathSuffix := "/chat"
	v1PostSessionChatPathIDParser := wf.PathIDParser(v1PostSessionChatPathSuffix)
	v1PostSessionChatPayloadParser := wf.JSONParser(reflect.TypeOf(ChatRequest{}))
	v1PostSessionChatMatcher := wf.ResourceWithID(http.MethodPost, "/v1/sessions/", v1PostSessionChatPathSuffix)
	v1PostSessionChatParser := func(data []byte, path string) (any, error) {
		id, err := v1PostSessionChatPathIDParser(nil, path)
		if err != nil {
			return nil, err
		}
		payload, err := v1PostSessionChatPayloadParser(data, "")
		if err != nil {
			return nil, err
		}
		return []any{id, payload}, nil
	}
	v1PostSessionChat := wf.NewClosureHandler(
		func(req *http.Request) bool {
			if !v1PostSessionChatMatcher(req) {
				return false
			}
			return req.URL.Query().Get("stream") != "true"
		},
		v1PostSessionChatParser,
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.Chat(ctx, req.([]any)[0].(int), req.([]any)[1].(*ChatRequest))
		},
		json.Marshal,
		wf.JSONContentType,
	)
	v1PostSessionChatStream := wf.NewServerSentEventsHandler(
		func(req *http.Request) bool {
			if !v1PostSessionChatMatcher(req) {
				return false
			}
			return req.URL.Query().Get("stream") == "true"
		},
		v1PostSessionChatParser,
		func(ctx context.Context, req any) (ch <-chan wf.MessageEvent, codedError *wf.CodedError) {
			return ret.ChatStream(ctx, req.([]any)[0].(int), req.([]any)[1].(*ChatRequest))
		},
	)

	ret.web = wf.NewWeb(
		false,
		v1PostSession,
		v1GetSessions,
		v1GetSessionByID,
		v1PostSessionChat,
		v1PostSessionChatStream,
	)
	return ret
}
