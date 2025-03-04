package service

import (
	"aiagent/clients/openai"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type Session struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Chats []*Chat `json:"chats"` // pointer item because item could be modified just after appending
}

func (s *Session) Info() SessionInfo {
	return SessionInfo{
		ID:   s.ID,
		Name: s.Name,
	}
}

func (s *Session) History() []openai.Message {
	var ret []openai.Message
	for _, chat := range s.Chats {
		if !chat.Valid() {
			continue
		}
		ret = append(ret, chat.HistoryRecords()...)
	}
	return ret
}

type Chat struct {
	Input   string                 `json:"input"`
	Created time.Time              `json:"created"`
	Result  *openai.ChatCompletion `json:"result"`
}

func (c Chat) Valid() bool {
	return c.Result != nil && len(c.Result.Choices) == 1 && c.Result.Choices[0].FinishReason == openai.FinishReasonStop
}

func (c Chat) HistoryRecords() []openai.Message {
	var ret []openai.Message
	ret = append(ret, openai.NewUserMessage(c.Input))
	ret = append(ret, c.Result.Choices[0].Message.HistoryRecord())
	return ret
}

type Service struct {
	client *openai.Client
	web    *Web
	data   map[int]*Session // pointer item as easier to edit after fetch
	mu     sync.Mutex       // guard key write action over data
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.web.ServeHTTP(writer, request)
}

func (s *Service) CreateSession(_ context.Context) (int, *CodedError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := len(s.data) + 1000
	s.data[id] = &Session{
		ID:    id,
		Name:  strconv.Itoa(id),
		Chats: nil,
	}
	return id, nil
}

type SessionInfo struct {
	ID   int
	Name string
}

func (s *Service) FindSessions(_ context.Context) ([]SessionInfo, *CodedError) {
	var ret []SessionInfo
	for _, session := range s.data {
		ret = append(ret, session.Info())
	}
	return ret, nil
}

func (s *Service) FindSessionByID(_ context.Context, id int) (*Session, *CodedError) {
	ret, ok := s.data[id]
	if !ok {
		return nil, NewCodedErrorf(http.StatusNotFound, "no session on id %v", id)
	}
	return ret, nil
}

type ChatRequest struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

func (s *Service) Chat(ctx context.Context, id int, req *ChatRequest) (*openai.ChatCompletion, *CodedError) {
	session, ok := s.data[id]
	if !ok {
		return nil, NewCodedErrorf(http.StatusNotFound, "no session on id to chat %v", id)
	}

	chat := &Chat{
		Input:   req.Content,
		Created: time.Now(),
		Result:  nil,
	}
	session.Chats = append(session.Chats, chat)
	// session.History() would skip the one just generated as Valid() does not meet because of nil Result,
	// so it's correct to append again to generate the history to be used in Request.
	chatCompletion, err := s.client.OneShot(ctx, openai.Request{
		Messages: append(session.History(), openai.NewUserMessage(req.Content)),
		Model:    req.Model,
	})
	if err != nil {
		return nil, NewCodedErrorf(http.StatusInternalServerError, "upstream: %v", err.Error())
	}
	chat.Result = chatCompletion

	return chatCompletion, nil
}

func New(
	client *openai.Client,
) *Service {
	ret := &Service{
		client: client,
		web:    nil,
		data:   make(map[int]*Session),
		mu:     sync.Mutex{},
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
