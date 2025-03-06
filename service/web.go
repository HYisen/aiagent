package service

import (
	"aiagent/clients/chat"
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"reflect"
	"time"
)

type Service struct {
	client *openai.Client
	web    *Web

	sessionRepository *session.Repository
	chatRepository    *chat.Repository
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.web.ServeHTTP(writer, request)
}

func (s *Service) CreateSession(ctx context.Context) (int, *CodedError) {
	name := time.Now().String()
	if err := s.sessionRepository.Save(ctx, model.Session{
		ID:   0,
		Name: name,
	}); err != nil {
		return 0, nil
	}
	id, err := s.sessionRepository.FindLastIDByUserIDAndName(ctx, 0, name)
	if err != nil {
		return 0, NewCodedError(http.StatusInternalServerError, err)
	}
	return id, nil
}

func (s *Service) FindSessions(ctx context.Context) ([]*model.Session, *CodedError) {
	ret, err := s.sessionRepository.FindAll(ctx)
	if err != nil {
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

func (s *Service) FindSessionByID(ctx context.Context, id int) (*model.Session, *CodedError) {
	ret, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewCodedErrorf(http.StatusNotFound, "no session on id %v", id)
	}
	if err != nil {
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

type ChatRequest struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

func (s *Service) Chat(ctx context.Context, id int, req *ChatRequest) (*openai.ChatCompletion, *CodedError) {
	ses, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NewCodedErrorf(http.StatusNotFound, "no session on id %v to chat", id)
	}
	if err != nil {
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}

	if err := s.chatRepository.Save(ctx, &model.Chat{
		ID:         0,
		SessionID:  ses.ID,
		Input:      req.Content,
		CreateTime: time.Now().UnixMilli(),
		Result:     nil,
	}); err != nil {
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}

	neo, err := s.chatRepository.FindLastBySessionID(ctx, ses.ID)
	if err != nil {
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}

	chatCompletion, err := s.client.OneShot(ctx, openai.Request{
		Messages: append(ses.History(), openai.NewUserMessage(req.Content)),
		Model:    req.Model,
	})
	if err != nil {
		return nil, NewCodedErrorf(http.StatusInternalServerError, "upstream: %v", err.Error())
	}
	neo.Result = model.NewResult(chatCompletion)

	if err := s.chatRepository.Save(ctx, neo); err != nil {
		slog.Error("can not append record", "chat", neo)
		return nil, NewCodedError(http.StatusInternalServerError, err)
	}
	return chatCompletion, nil
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

	v1PostSession := NewJSONHandler(
		Exact(http.MethodPost, "/v1/sessions"),
		reflect.TypeOf(Empty{}),
		func(ctx context.Context, req any) (rsp any, codedError *CodedError) {
			return ret.CreateSession(ctx)
		},
	)

	v1GetSessions := NewJSONHandler(
		Exact(http.MethodGet, "/v1/sessions"),
		reflect.TypeOf(Empty{}),
		func(ctx context.Context, req any) (rsp any, codedError *CodedError) {
			return ret.FindSessions(ctx)
		},
	)

	v1GetSessionByID := &ClosureHandler{
		Matcher: ResourceWithID(http.MethodGet, "/v1/sessions/", ""),
		Parser:  PathIDParser(""),
		Handler: func(ctx context.Context, req any) (rsp any, codedError *CodedError) {
			return ret.FindSessionByID(ctx, req.(int))
		},
		Formatter:   json.Marshal,
		ContentType: JSONContentType,
	}

	v1PostSessionChatPathSuffix := "/chat"
	v1PostSessionChatPathIDParser := PathIDParser(v1PostSessionChatPathSuffix)
	v1PostSessionChatPayloadParser := JSONParser(reflect.TypeOf(ChatRequest{}))
	v1PostSessionChat := &ClosureHandler{
		Matcher: ResourceWithID(http.MethodPost, "/v1/sessions/", v1PostSessionChatPathSuffix),
		Parser: func(data []byte, path string) (any, error) {
			id, err := v1PostSessionChatPathIDParser(nil, path)
			if err != nil {
				return nil, err
			}
			payload, err := v1PostSessionChatPayloadParser(data, "")
			if err != nil {
				return nil, err
			}
			return []any{id, payload}, nil
		},
		Handler: func(ctx context.Context, req any) (rsp any, codedError *CodedError) {
			return ret.Chat(ctx, req.([]any)[0].(int), req.([]any)[1].(*ChatRequest))
		},
		Formatter:   json.Marshal,
		ContentType: JSONContentType,
	}

	ret.web = NewWeb(
		true,
		v1PostSession,
		v1GetSessions,
		v1GetSessionByID,
		v1PostSessionChat,
	)
	return ret
}
