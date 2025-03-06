package chat

import (
	"aiagent/clients/model"
	"aiagent/clients/query"
	"context"
	"gorm.io/gorm"
)

type Repository struct {
	q *query.Query
}

func NewRepository(d gorm.Dialector) (*Repository, error) {
	db, err := gorm.Open(d)
	if err != nil {
		return nil, err
	}
	return &Repository{
		q: query.Use(db),
	}, nil
}

func (r *Repository) FindBySessionID(ctx context.Context, sessionID int) ([]*model.Chat, error) {
	return r.q.Chat.WithContext(ctx).Preload(r.q.Chat.Result).Where(r.q.Chat.SessionID.Eq(sessionID)).Find()
}
