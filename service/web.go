package service

import (
	"aiagent/clients/chat"
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	"context"
	"encoding/json"
	"errors"
	"github.com/hyisen/wf"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"reflect"
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
	ses, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v to chat", id)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	if err := s.chatRepository.Save(ctx, &model.Chat{
		ID:         0,
		SessionID:  ses.ID,
		Input:      req.Content,
		CreateTime: time.Now().UnixMilli(),
		Result:     nil,
	}); err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	neo, err := s.chatRepository.FindLastBySessionID(ctx, ses.ID)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
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
	v1PostSessionChat := wf.NewClosureHandler(
		wf.ResourceWithID(http.MethodPost, "/v1/sessions/", v1PostSessionChatPathSuffix),
		func(data []byte, path string) (any, error) {
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
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.Chat(ctx, req.([]any)[0].(int), req.([]any)[1].(*ChatRequest))
		},
		json.Marshal,
		wf.JSONContentType,
	)

	ret.web = wf.NewWeb(
		false,
		v1PostSession,
		v1GetSessions,
		v1GetSessionByID,
		v1PostSessionChat,
	)
	return ret
}
