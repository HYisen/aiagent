package console

import (
	"fmt"
	"sync/atomic"
)

type MultiLineRemote struct {
	enabled *atomic.Bool
	symbol  string
}

func (r *MultiLineRemote) ExitMultiLineHint() string {
	return fmt.Sprintf(`Inside multi-line mode, type "%s" in new line to end multi-line input.`, r.symbol)
}

func NewMultiLineRemote(symbol string) *MultiLineRemote {
	return &MultiLineRemote{
		enabled: &atomic.Bool{},
		symbol:  symbol,
	}
}

func (r *MultiLineRemote) EnableMultiLineOnce() {
	r.enabled.Store(true)
}

// OnMultiLineEndExit returns whether the input line shall trigger multi-line mode end.
// If it would, the multi-line status controlled by receiver will be set to false.
func (r *MultiLineRemote) OnMultiLineEndExit(line string) bool {
	if line == r.symbol {
		r.enabled.Store(false)
		return true
	}
	return false
}

// MultiLine returns whether the multi-line status controlled by receiver is enabled.
func (r *MultiLineRemote) MultiLine() bool {
	return r.enabled.Load()
}

// NotMultiLineChecker provides a [console.MultiLineChecker] if you would never want to enter multi-line mode.
// A zero value works, so NewNotMultiLineChecker does not exist.
// This struct is designed to free you from using [console.NewMultiLineRemote] with actually impossible symbol.
type NotMultiLineChecker struct {
}

func (c *NotMultiLineChecker) OnMultiLineEndExit(_ string) bool {
	return false
}

func (c *NotMultiLineChecker) MultiLine() bool {
	return false
}
