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

func NewRepository(db *gorm.DB) (*Repository, error) {
	return &Repository{
		q: query.Use(db),
	}, nil
}

func (r *Repository) FindBySessionID(ctx context.Context, sessionID int) ([]*model.Chat, error) {
	return r.q.Chat.WithContext(ctx).Preload(r.q.Chat.Result).Where(r.q.Chat.SessionID.Eq(sessionID)).Find()
}

func (r *Repository) Save(ctx context.Context, chat *model.Chat) error {
	return r.q.Chat.WithContext(ctx).Save(chat)
}

func (r *Repository) FindLastBySessionID(ctx context.Context, sessionID int) (*model.Chat, error) {
	return r.q.Chat.WithContext(ctx).Where(r.q.Chat.SessionID.Eq(sessionID)).Last()
}
