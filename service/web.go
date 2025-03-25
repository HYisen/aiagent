package service

import (
	"aiagent/clients/chat"
	"aiagent/clients/model"
	"aiagent/clients/openai"
	"aiagent/clients/session"
	sc "aiagent/service/chat"
	"context"
	"encoding/json"
	"errors"
	"github.com/hyisen/wf"
	"gorm.io/gorm"
	"net/http"
	"reflect"
)

type Service struct {
	v2          *V2Service
	chatService *sc.Service
	web         *wf.Web

	sessionRepository *session.Repository
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.web.ServeHTTP(writer, request)
}

func (s *Service) CreateSession(ctx context.Context) (int, *wf.CodedError) {
	name := model.DefaultSessionName()
	if err := s.sessionRepository.Save(ctx, model.Session{
		ID:   0,
		Name: name,
	}); err != nil {
		return 0, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	id, err := s.sessionRepository.FindLastIDByName(ctx, name)
	if err != nil {
		return 0, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return id, nil
}

func (s *Service) FindSessions(ctx context.Context) ([]*model.SessionWithID, *wf.CodedError) {
	items, err := s.sessionRepository.FindAll(ctx)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	var ret []*model.SessionWithID
	for _, item := range items {
		ret = append(ret, item.SessionWithID())
	}
	return ret, nil
}

func (s *Service) FindSessionByID(ctx context.Context, id int) (*model.SessionWithID, *wf.CodedError) {
	ret, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v", id)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret.SessionWithID(), nil
}

func New(
	client *openai.Client,
	sessionRepository *session.Repository,
	chatRepository *chat.Repository,
) *Service {
	ret := &Service{
		web:               nil,
		v2:                NewV2Service(sessionRepository),
		chatService:       sc.NewService(client, chatRepository, sessionRepository),
		sessionRepository: sessionRepository,
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
	v1PostSessionChatPayloadParser := wf.JSONParser(reflect.TypeOf(sc.RequestPayload{}))
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
			return ret.chatService.ChatSimple(ctx, req.([]any)[0].(int), req.([]any)[1].(*sc.RequestPayload))
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
			return ret.chatService.ChatStream(ctx, req.([]any)[0].(int), req.([]any)[1].(*sc.RequestPayload))
		},
	)

	v2SessionsPathSuffix := "/sessions"
	v2GetSessions := wf.NewClosureHandler(
		wf.ResourceWithID(http.MethodGet, "/v2/users/", v2SessionsPathSuffix),
		wf.PathIDParser(v2SessionsPathSuffix),
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.v2.FindSessionsByUserID(ctx, req.(int))
		},
		json.Marshal,
		wf.JSONContentType,
	)

	v2PostSessionMatcher, v2PostSessionParser := wf.ResourceWithIDs(
		http.MethodPost,
		[]string{"v2", "users", "", "sessions"},
	)
	v2PostSession := wf.NewClosureHandler(
		v2PostSessionMatcher,
		v2PostSessionParser,
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.v2.CreateSessionByUserID(ctx, req.([]int)[0])
		}, json.Marshal,
		wf.JSONContentType,
	)

	v2GetSessionMatcher, v2GetSessionParser := wf.ResourceWithIDs(
		http.MethodGet,
		[]string{"v2", "users", "", "sessions", ""},
	)
	v2GetSession := wf.NewClosureHandler(
		v2GetSessionMatcher,
		v2GetSessionParser,
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			ids := req.([]int)
			userID, scopedID := ids[0], ids[1]
			return ret.v2.FindSession(ctx, userID, scopedID)
		},
		json.Marshal,
		wf.JSONContentType,
	)

	v2PostSessionChatPathMatcher, v2PostSessionChatPathParser := wf.ResourceWithIDs(
		http.MethodPost,
		[]string{"v2", "users", "", "sessions", "", "chat"},
	)
	v2PostSessionChat := wf.NewClosureHandler(
		func(req *http.Request) bool {
			if !v2PostSessionChatPathMatcher(req) {
				return false
			}
			return req.URL.Query().Get("stream") != "true"
		},
		func(data []byte, path string) (req any, err error) {
			raw, err := v2PostSessionChatPathParser(nil, path)
			if err != nil {
				return nil, err
			}
			ids := raw.([]int)
			request := &sc.Request{
				UserID:          ids[0],
				SessionScopedID: ids[1],
				RequestPayload:  sc.RequestPayload{},
			}
			if err := json.Unmarshal(data, &request.RequestPayload); err != nil {
				return nil, err
			}
			return request, nil
		},
		func(ctx context.Context, req any) (rsp any, codedError *wf.CodedError) {
			return ret.chatService.Chat(ctx, req.(*sc.Request))
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
		v1PostSessionChatStream,
		v2GetSessions,
		v2PostSession,
		v2GetSession,
		v2PostSessionChat,
	)
	return ret
}
