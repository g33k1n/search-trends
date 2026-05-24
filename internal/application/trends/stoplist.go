package trends

import (
	"context"

	domain "github.com/leva/search-trends/internal/domain/trends"
)

type StopListUseCase struct {
	store TrendsStore
}

func NewStopListUseCase(store TrendsStore) *StopListUseCase {
	return &StopListUseCase{store: store}
}

func (uc *StopListUseCase) Add(ctx context.Context, raw string) error {
	query, err := domain.NewSearchQuery(raw)
	if err != nil {
		return err
	}
	return uc.store.AddStop(ctx, query)
}

func (uc *StopListUseCase) Remove(ctx context.Context, raw string) error {
	query, err := domain.NewSearchQuery(raw)
	if err != nil {
		return err
	}
	return uc.store.RemoveStop(ctx, query)
}

func (uc *StopListUseCase) List(ctx context.Context) []string {
	return uc.store.ListStop(ctx)
}
