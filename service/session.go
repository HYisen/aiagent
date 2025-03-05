package service

import (
	"aiagent/clients/model"
	"aiagent/clients/openai"
)

type Session struct {
	ID    int            `json:"id"`
	Name  string         `json:"name"`
	Chats []*openai.Chat `json:"chats"` // pointer item because item could be modified just after appending
}

func NewSession(session *model.Session, chats []*model.Chat) *Session {
	var items []*openai.Chat
	for _, c := range chats {
		items = append(items, c.Chat())
	}
	return &Session{
		ID:    session.ID,
		Name:  session.Name,
		Chats: items,
	}
}

func (s *Session) Info() model.Session {
	return model.Session{
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
