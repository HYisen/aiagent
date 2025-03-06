package session

import (
	"aiagent/clients/model"
	"aiagent/clients/query"
	"context"
	"errors"
	"fmt"
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

func (r *Repository) FindAll(ctx context.Context) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Find()
}

func (r *Repository) Find(ctx context.Context, id int) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.ID.Eq(id)).First()
}

func (r *Repository) FindWithChats(ctx context.Context, id int) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).
		Where(r.q.Session.ID.Eq(id)).
		Preload(r.q.Session.Chats).
		Preload(r.q.Session.Chats.Result).
		First()
}

func (r *Repository) Save(ctx context.Context, item model.Session) error {
	return r.q.Session.WithContext(ctx).Save(&item)
}

func (r *Repository) FindLastIDByUserIDAndName(ctx context.Context, userID int, name string) (int, error) {
	do := r.q.Session.WithContext(ctx)
	if userID != 0 {
		do = do.Where(r.q.Session.UserID.Eq(userID))
	}
	last, err := do.Where(r.q.Session.Name.Eq(name)).Last()
	if err != nil {
		return 0, err
	}
	return last.ID, err
}

// AppendChat is unsupported yet.
func (r *Repository) AppendChat(_ context.Context, _ int, _ *model.Chat) error {
	// the implementation is dropped because upstream lacks feature of append with cascading associations
	// ref https://github.com/go-gorm/gen/issues/1242
	// in short, the flowing commented code would fail with nil chat.Result in DB
	// return r.q.Session.Chats.WithContext(ctx).Model(&model.Session{ID: sessionID}).Append(chat)
	return fmt.Errorf("AppendChat %w", errors.ErrUnsupported)
}
