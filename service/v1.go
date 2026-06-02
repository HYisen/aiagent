package service

import (
	"aiagent/clients/model"
	"aiagent/clients/session"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hyisen/wf"
	"gorm.io/gorm"
)

// V1Service provides abilities without scoped_id constraint,
// which makes it ideal for administrator actions, but not for normal users.
// The V1 name does come from its earlier adoption,
// but do not mean to be deprecated now nor in the future.
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

func (s *V1Service) CleanEmptyOldSession(ctx context.Context) *wf.CodedError {
	idToName, err := s.sessionRepository.FindEmptySessionIDToName(ctx)
	if err != nil {
		return wf.NewCodedError(http.StatusInternalServerError, err)
	}
	ids := filterOldAndMapSessionIDs(idToName)
	err = s.sessionRepository.DeleteByIDs(ctx, ids...)
	if err != nil {
		return wf.NewCodedError(http.StatusInternalServerError, err)
	}
	return nil
}

func filterOldAndMapSessionIDs(sessionIDToName map[int]string) (sessionIDs []int) {
	var ret []int
	// Define sessions created such long time ago without chats are orphaned.
	threshold := time.Now().Add(-time.Hour * 4)
	for id, name := range sessionIDToName {
		t, generatedFromTime := model.DigestSessionName(name)
		if !generatedFromTime {
			continue
		}
		if t.Before(threshold) {
			ret = append(ret, id)
		}
	}
	return ret
}
