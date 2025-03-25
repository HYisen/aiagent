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

func (r *Repository) FindByUserID(ctx context.Context, userID int) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.UserID.Eq(userID)).Find()
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

func (r *Repository) FindWithChatsByUserIDAndScopedID(ctx context.Context, userID int, scopedID int) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).
		Where(r.q.Session.UserID.Eq(userID)).
		Where(r.q.Session.ScopedID.Eq(scopedID)).
		Preload(r.q.Session.Chats).
		Preload(r.q.Session.Chats.Result).
		First()
}

func (r *Repository) Save(ctx context.Context, item model.Session) error {
	return r.q.Session.WithContext(ctx).Save(&item)
}

func (r *Repository) Create(ctx context.Context, userID int, name string) error {
	return r.q.Transaction(func(tx *query.Query) error {
		users, err := tx.User.WithContext(ctx).Where(tx.User.ID.Eq(userID)).Find()
		if err != nil {
			return err
		}
		var scopedID int
		if len(users) == 0 {
			// 404 means we have lost synchronization with its user-auth module,
			// which could be a designed behaviour as lazy sync.
			// Because it passed auth, here we trust it. Create a place-holder user.
			if err := tx.User.WithContext(ctx).Create(&model.User{
				ID:               0,
				Nickname:         "auto",
				SessionsSequence: scopedID,
			}); err != nil {
				return err
			}
		} else {
			scopedID = users[0].SessionsSequence
		}
		scopedID++
		if _, err := tx.User.WithContext(ctx).
			Where(tx.User.ID.Eq(userID)).
			Update(tx.User.SessionsSequence, scopedID); err != nil {
			return err
		}

		return tx.Session.WithContext(ctx).Create(&model.Session{
			ID:       0,
			Name:     name,
			UserID:   userID,
			ScopedID: scopedID,
			Chats:    nil,
		})
	})
}

func (r *Repository) FindLastIDByName(ctx context.Context, name string) (int, error) {
	last, err := r.q.Session.WithContext(ctx).Where(r.q.Session.Name.Eq(name)).Last()
	if err != nil {
		return 0, err
	}
	return last.ID, err
}

func (r *Repository) FindLastByUserIDAndName(ctx context.Context, userID int, name string) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).
		Where(r.q.Session.UserID.Eq(userID)).
		Where(r.q.Session.Name.Eq(name)).
		Last()
}

// AppendChat is unsupported yet.
func (r *Repository) AppendChat(_ context.Context, _ int, _ *model.Chat) error {
	// the implementation is dropped because upstream lacks feature of append with cascading associations
	// ref https://github.com/go-gorm/gen/issues/1242
	// in short, the flowing commented code would fail with nil chat.Result in DB
	// return r.q.Session.Chats.WithContext(ctx).Model(&model.Session{ID: sessionID}).Append(chat)
	return fmt.Errorf("AppendChat %w", errors.ErrUnsupported)
}
