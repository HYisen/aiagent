package service

import (
	"aiagent/clients/model"
	"aiagent/clients/session"
	"context"
	"errors"
	"github.com/hyisen/wf"
	"gorm.io/gorm"
	"net/http"
)

type V2Service struct {
	sessionRepository *session.Repository
}

func NewV2Service(sessionRepository *session.Repository) *V2Service {
	return &V2Service{sessionRepository: sessionRepository}
}

func (s *V2Service) CreateSessionByUserID(ctx context.Context, userID int) (created *model.Session, _ *wf.CodedError) {
	name := model.DefaultSessionName()
	if err := s.sessionRepository.Create(ctx, userID, name); err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	ret, err := s.sessionRepository.FindLastByUserIDAndName(ctx, userID, name)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

func (s *V2Service) FindSessionsByUserID(ctx context.Context, userID int) ([]*model.Session, *wf.CodedError) {
	ret, err := s.sessionRepository.FindByUserID(ctx, userID)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}

func (s *V2Service) FindSession(ctx context.Context, userID int, scopedID int) (*model.Session, *wf.CodedError) {
	ret, err := s.sessionRepository.FindWithChatsByUserIDAndScopedID(ctx, userID, scopedID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session at %v-%v", userID, scopedID)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret, nil
}
