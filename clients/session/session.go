package session

import (
	"aiagent/clients/model"
	"aiagent/clients/query"
	"context"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

type Repository struct {
	q *query.Query
}

func NewRepository() *Repository {
	db, err := gorm.Open(sqlite.Open("db"))
	if err != nil {
		log.Fatalln(err)
	}
	return &Repository{
		q: query.Use(db),
	}
}

func (r *Repository) FindAll(ctx context.Context) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Find()
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
