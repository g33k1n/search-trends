package trends

import (
	"context"
	"time"

	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type IngestEventCommand struct {
	Query     string
	Timestamp time.Time
}

type IngestEventUseCase struct {
	store   TrendsStore
	metrics MetricsSink
}

func NewIngestEventUseCase(store TrendsStore, metrics MetricsSink) *IngestEventUseCase {
	return &IngestEventUseCase{store: store, metrics: metrics}
}

func (uc *IngestEventUseCase) Handle(ctx context.Context, cmd IngestEventCommand) error {
	query, err := domain.NewSearchQuery(cmd.Query)
	if err != nil {
		uc.metrics.EventRejected()
		return err
	}
	if err := uc.store.Add(ctx, query, cmd.Timestamp); err != nil {
		uc.metrics.EventRejected()
		return err
	}
	uc.metrics.EventAccepted()
	return nil
}
