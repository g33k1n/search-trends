package trends

import (
	"strings"
	"time"
)

type SearchQuery struct {
	value string
}

func NewSearchQuery(raw string) (SearchQuery, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return SearchQuery{}, ErrEmptyQuery
	}
	return SearchQuery{value: normalized}, nil
}

func (q SearchQuery) String() string { return q.value }

func (q SearchQuery) IsZero() bool { return q.value == "" }

type SearchEvent struct {
	Query     SearchQuery
	UserID    string
	RequestID string
	Timestamp time.Time
	Source    string
}
