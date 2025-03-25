package service

import (
	"aiagent/clients/chat"
	"aiagent/clients/model"
	"aiagent/clients/session"
	"context"
	"github.com/hyisen/wf"
	"net/http"
)

type V2Service struct {
	sessionRepository *session.Repository
	chatRepository    *chat.Repository
}

func NewV2Service(chatRepository *chat.Repository, sessionRepository *session.Repository) *V2Service {
	return &V2Service{chatRepository: chatRepository, sessionRepository: sessionRepository}
}

func (s *V2Service) CreateSessionByUserID(ctx context.Context, userID int) (int, *wf.CodedError) {
	name := model.DefaultSessionName()
	if err := s.sessionRepository.Create(ctx, userID, name); err != nil {
		return 0, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	id, err := s.sessionRepository.FindLastIDByUserIDAndName(ctx, userID, name)
	if err != nil {
		return 0, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return id, nil
}

func (s *V2Service) FindSessionsByUserID(ctx context.Context, userID int) ([]*model.Session, *wf.CodedError) {
	ret, err := s.sessionRepository.FindByUserID(ctx, userID)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}
