// File meta.go contains only necessary code to limit GORM CLI reachability.
// Some code don't have to be, even can't be used by the generator.
// This is an allow list, secondary code such as methods shall be elsewhere if could.

//go:generate go tool gorm gen -i meta.go -o ../generated

package model

//goland:noinspection GoCommentStart
type SessionChatQuery[T any] interface {
	// SELECT chats.id, session_id, create_time
	// FROM chats
	// LEFT JOIN sessions ON chats.session_id = sessions.id
	// WHERE user_id = @userID
	// ORDER BY chats.session_id;
	FindChatPartByUserID(userID int) ([]ChatPart, error)
}
