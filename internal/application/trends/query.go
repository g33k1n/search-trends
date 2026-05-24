package trends

import (
	"context"
	"time"

	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type TopQuery struct {
	Limit int
}

type TopResult struct {
	Window time.Duration
	Items  []domain.TrendEntry
}

type QueryTopUseCase struct {
	store TrendsStore
}

func NewQueryTopUseCase(store TrendsStore) *QueryTopUseCase {
	return &QueryTopUseCase{store: store}
}

func (uc *QueryTopUseCase) Handle(ctx context.Context, q TopQuery) TopResult {
	if q.Limit <= 0 {
		q.Limit = 10
	}
	return TopResult{
		Window: uc.store.Window(),
		Items:  uc.store.Top(ctx, q.Limit),
	}
}
