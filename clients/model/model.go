// Package model stores models used by gen.
// It's not mandatory, just comparing to the other choice to implements GenInternalDoName for many times,
// I would rather pull all the structs needed together here.
// ref https://github.com/go-gorm/gen/issues/971
package model

type Session struct {
	ID     int
	Name   string
	UserID int
}
