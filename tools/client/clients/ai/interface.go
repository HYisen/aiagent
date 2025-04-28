package ai

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
}
