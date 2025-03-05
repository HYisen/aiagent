package session

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

func (r *Repository) FindAll(ctx context.Context) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Find()
}

func (r *Repository) Find(ctx context.Context, id int) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.ID.Eq(id)).First()
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
