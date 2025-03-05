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
	// It's this the right way to use gen Associations?
	chats, err := r.q.Chat.WithContext(ctx).Where(r.q.Chat.SessionID.Eq(sessionID)).Find()
	if err != nil {
		return nil, err
	}
	for _, chat := range chats {
		result, err := r.q.Chat.Result.WithContext(ctx).Model(&model.Chat{ID: chat.ID}).Find()
		if err != nil {
			return nil, err
		}
		chat.Result = result
	}
	return chats, err
}
