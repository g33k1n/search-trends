package trends

import (
	"context"
	"time"

	domain "github.com/leva/search-trends/internal/domain/trends"
)

type TrendsStore interface {
	Add(ctx context.Context, query domain.SearchQuery, at time.Time) error
	Top(ctx context.Context, limit int) []domain.TrendEntry
	AddStop(ctx context.Context, query domain.SearchQuery) error
	RemoveStop(ctx context.Context, query domain.SearchQuery) error
	ListStop(ctx context.Context) []string
	Window() time.Duration
}

type MetricsSink interface {
	EventReceived()
	EventAccepted()
	EventRejected()
	EventMalformed()
}
