package service

import (
	"aiagent/clients/model"
	"aiagent/clients/session"
	"context"
	"errors"
	"net/http"

	"github.com/hyisen/wf"
	"gorm.io/gorm"
)

type V1Service struct {
	sessionRepository *session.Repository
}

func NewV1Service(sessionRepository *session.Repository) *V1Service {
	return &V1Service{sessionRepository: sessionRepository}
}

func (s *V1Service) CreateSession(ctx context.Context) (int, *wf.CodedError) {
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

func (s *V1Service) FindSessions(ctx context.Context) ([]*model.SessionWithChatsDigestAndID, *wf.CodedError) {
	items, err := s.sessionRepository.FindAll(ctx)
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}

	var ret []*model.SessionWithChatsDigestAndID
	for _, item := range items {
		ret = append(ret, item.WithID())
	}
	return ret, nil
}

func (s *V1Service) FindSessionByID(ctx context.Context, id int) (*model.SessionWithID, *wf.CodedError) {
	ret, err := s.sessionRepository.FindWithChats(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wf.NewCodedErrorf(http.StatusNotFound, "no session on id %v", id)
	}
	if err != nil {
		return nil, wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return ret.WithID(), nil
}
