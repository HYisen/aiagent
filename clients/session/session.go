package session

import (
	"aiagent/clients/generated"
	"aiagent/clients/model"
	"aiagent/clients/query"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	q  *query.Query
	db *gorm.DB
}

func NewRepository(db *gorm.DB) (*Repository, error) {
	return &Repository{
		q:  query.Use(db),
		db: db,
	}, nil
}

func (r *Repository) FindAll(ctx context.Context) ([]*model.SessionWithChatsDigest, error) {
	sessions, err := gorm.G[*model.Session](r.db).Find(ctx)
	if err != nil {
		return nil, err
	}

	var chats []model.ChatPart
	if err := r.db.WithContext(ctx).Model(&model.Chat{}).Find(&chats).Error; err != nil {
		return nil, err
	}

	return extendSessionWithChatsDigest(sessions, digest(chats)), nil
}

func (r *Repository) FindByUserID(ctx context.Context, userID int) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.UserID.Eq(userID)).Find()
}

func (r *Repository) FindIDByUserIDAndScopedID(ctx context.Context, userID int, scopedID int) (int, error) {
	item, err := r.q.Session.WithContext(ctx).
		Where(r.q.Session.UserID.Eq(userID)).
		Where(r.q.Session.ScopedID.Eq(scopedID)).
		Select(r.q.Session.ID).
		First()
	if err != nil {
		return 0, err
	}
	return item.ID, nil
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
			// which could be a designed behavior as lazy sync.
			// Because it passed auth, here we trust it. Create a place-holder user.
			if err := tx.User.WithContext(ctx).Create(&model.User{
				ID:               userID,
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
	last, err := r.q.Session.WithContext(ctx).Where(r.q.Session.Name.Eq(name)).Select(r.q.Session.ID).Last()
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
	// The implementation is dropped because upstream lacks the feature of appending with cascading associations
	// See https://github.com/go-gorm/gen/issues/1242
	// In short, the flowing commented code would fail with nil chat.Result in DB.
	// return r.q.Session.Chats.WithContext(ctx).Model(&model.Session{ID: sessionID}).Append(chat)
	return fmt.Errorf("AppendChat %w", errors.ErrUnsupported)
}

type ChatsDigests map[int]*model.ChatsDigest

func (cd ChatsDigests) GetOrDefault(sessionID int) model.ChatsDigest {
	ret, ok := cd[sessionID]
	if !ok {
		var keys string
		if len(cd) > 8 { // A threshold within the ability of human visual check and log capacity.
			keys = fmt.Sprintf("len=%d", len(cd))
		} else {
			keys = fmt.Sprintf("%v", slices.Collect(maps.Keys(cd)))
		}
		// CreateSession and its chats' creation are separate procedures.
		// Session with empty chats are planned to be cleaned, but not guaranteed to be extinct.
		slog.Warn("unmatched SessionChatsDigests", "sessionID", sessionID, "keys", keys)
		dummyTime := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
		return model.ChatsDigest{
			Rounds:               0,
			CreateTimeEpochMilli: dummyTime,
			UpdateTimeEpochMilli: dummyTime,
		}
	}
	return *ret
}

func digest(chats []model.ChatPart) ChatsDigests {
	farFuture := time.Now().Add(time.Hour) // Later than any possible chat time fetched later.
	sessionIDToChatsDigests := make(map[int]*model.ChatsDigest)
	for _, part := range chats {
		if sessionIDToChatsDigests[part.SessionID] == nil {
			sessionIDToChatsDigests[part.SessionID] = &model.ChatsDigest{
				Rounds:               0,
				CreateTimeEpochMilli: farFuture.UnixMilli(),
				UpdateTimeEpochMilli: 0, // assert no chat happened before Genesis
			}
		}
		sessionIDToChatsDigests[part.SessionID].Rounds++
		sessionIDToChatsDigests[part.SessionID].CreateTimeEpochMilli =
			min(sessionIDToChatsDigests[part.SessionID].CreateTimeEpochMilli, part.CreateTime)
		sessionIDToChatsDigests[part.SessionID].UpdateTimeEpochMilli =
			max(sessionIDToChatsDigests[part.SessionID].UpdateTimeEpochMilli, part.CreateTime)
	}
	return sessionIDToChatsDigests
}

func extendSessionWithChatsDigest(sessions []*model.Session, cd ChatsDigests) []*model.SessionWithChatsDigest {
	var ret []*model.SessionWithChatsDigest
	for _, s := range sessions {
		ret = append(ret, &model.SessionWithChatsDigest{
			Session:     *s,
			ChatsDigest: cd.GetOrDefault(s.ID),
		})
	}
	return ret
}

func (r *Repository) FindWithChatsDigestByUserID(
	ctx context.Context,
	userID int,
) ([]*model.SessionWithChatsDigest, error) {
	chats, err := generated.SessionChatQuery[any](r.db).FindChatPartByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	sessions, err := r.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return extendSessionWithChatsDigest(sessions, digest(chats)), nil
}

func (r *Repository) FindEmptySessionIDToName(ctx context.Context) (sessionIDToName map[int]string, err error) {
	var nonEmptySessionIDs []int
	if err := r.q.Chat.WithContext(ctx).
		Distinct(r.q.Chat.SessionID).
		Pluck(r.q.Chat.SessionID, &nonEmptySessionIDs); err != nil {
		return nil, err
	}

	sessions, err := r.q.Session.WithContext(ctx).
		Where(r.q.Session.ID.NotIn(nonEmptySessionIDs...)).
		Select(r.q.Session.ID, r.q.Session.Name).Find()
	if err != nil {
		return nil, err
	}
	return extractSessionIDToName(sessions), nil
}

func extractSessionIDToName(sessions []*model.Session) (sessionIDToName map[int]string) {
	sessionIDToName = make(map[int]string)
	for _, session := range sessions {
		// session_id is PK, it's a bijection, can't override or lost item.
		sessionIDToName[session.ID] = session.Name
	}
	return sessionIDToName
}

func (r *Repository) DeleteByIDs(ctx context.Context, ids ...int) error {
	rowsAffected, err := gorm.G[model.Session](r.db).Where(generated.Session.ID.In(ids...)).Delete(ctx)
	if err == nil {
		slog.Info("deleted sessions", "count", rowsAffected, "ids", ids)
	}
	return err
}

func (r *Repository) UpdateName(ctx context.Context, id int, name string) error {
	_, err := gorm.G[model.Session](r.db).
		Where(generated.Session.ID.Eq(id)).
		Set(generated.Session.Name.Set(name)).
		Update(ctx)
	return err
}
