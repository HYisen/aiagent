package matcher

import (
	"fmt"
	"strconv"
	"strings"
)

type Matcher struct {
	all    bool
	exacts []int
	ranges [][2]int
}

func (m *Matcher) Match(value int) bool {
	if m.all {
		return true
	}
	for _, v := range m.exacts {
		if v == value {
			return true
		}
	}
	for _, r := range m.ranges {
		if value >= r[0] && value < r[1] {
			return true
		}
	}
	return false
}

func Parse(s string) (*Matcher, error) {
	if s == "*" {
		return &Matcher{
			all:    true,
			exacts: nil,
			ranges: nil,
		}, nil
	}

	var exact []int
	var ranges [][2]int
	for part := range strings.SplitSeq(s, ",") {
		r, err, ok := tryParseRangeString(strings.TrimSpace(part))
		if ok {
			if err != nil {
				return nil, fmt.Errorf("parse range %s: %w", part, err)
			}
			ranges = append(ranges, r)
		} else {
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("parse exact %s: %w", part, err)
			}
			exact = append(exact, num)
		}
	}
	return &Matcher{
		all:    false,
		exacts: exact,
		ranges: ranges,
	}, nil
}

func tryParseRangeString(s string) (rangeBeginInclusiveEndExclusive [2]int, err error, isRangeString bool) {
	before, after, found := strings.Cut(s, "-")
	if !found {
		return [2]int{}, nil, false
	}
	head, err := strconv.Atoi(before)
	if err != nil {
		return [2]int{}, err, true
	}
	tail, err := strconv.Atoi(after)
	if err != nil {
		return [2]int{}, err, true
	}
	return [2]int{head, tail + 1}, nil, true
}
