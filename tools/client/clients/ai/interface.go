package ai

import (
	"aiagent/clients/model"
	"runtime/debug"
	"slices"
)

// Client is the interface that what package client provides implements.
// Its user may prefer to define the interface on the caller side,
// but the interface is self-referenced, and return *ImplStruct does NOT match return Interface.
// Since callee should not, and cannot have a dependency of its caller, I have to provide the interface here
// and break the PECS creed as return an interface rather than an implementation type.
type Client interface {
	CreateSession() (id int, error error)
	Chat(sessionID int, content string) (words <-chan string, err error)
	// UpgradeOptional return nil Client and nil error when does not support upgrade,
	// otherwise a receiver's replacement.
	UpgradeOptional() (neo Client, err error)
	ListSessions() ([]Session, error)
	GetVersion() (version *debug.BuildInfo, err error)
}

// Session flats the difference between its implements [v1Session] and [v2Session],
// so that we could use them as one type.
type Session interface {
	IDValue() int
	IDField() string
	SessionCommon() SessionWithoutID
}

type SessionWithoutID struct {
	Name string
	model.ChatsDigest
}

func castUp[ActualType Session](items []ActualType) []Session {
	return slices.Collect(func(yield func(session Session) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	})
}
