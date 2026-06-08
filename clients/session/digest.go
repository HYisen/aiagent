package session

import (
	"aiagent/clients/generated"
	"aiagent/clients/model"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"gorm.io/gorm"
)

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

func (r *Repository) Find(ctx context.Context, id int) (*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.ID.Eq(id)).First()
}

func (r *Repository) FindByUserID(ctx context.Context, userID int) ([]*model.Session, error) {
	return r.q.Session.WithContext(ctx).Where(r.q.Session.UserID.Eq(userID)).Find()
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
