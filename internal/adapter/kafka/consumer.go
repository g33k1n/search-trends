package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	apptrends "github.com/leva/search-trends/internal/application/trends"
)

type Config struct {
	Brokers []string
	Topic   string
	GroupID string
}

type eventDTO struct {
	Query     string    `json:"query"`
	UserID    string    `json:"user_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
}

type Consumer struct {
	client    *kgo.Client
	ingest    *apptrends.IngestEventUseCase
	metrics   apptrends.MetricsSink
	logger    *slog.Logger
	wg        sync.WaitGroup
	closeOnce sync.Once
	cancel    context.CancelFunc
}

func Start(
	ctx context.Context,
	cfg Config,
	ingest *apptrends.IngestEventUseCase,
	metrics apptrends.MetricsSink,
	logger *slog.Logger,
) (*Consumer, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	if cfg.Topic == "" {
		return nil, errors.New("kafka topic is required")
	}
	if cfg.GroupID == "" {
		return nil, errors.New("kafka group id is required")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(5*time.Second),
		kgo.SessionTimeout(30*time.Second),
		kgo.WithLogger(kgo.BasicLogger(slogWriter{logger: logger}, kgo.LogLevelInfo, nil)),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka new client: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	c := &Consumer{
		client:  client,
		ingest:  ingest,
		metrics: metrics,
		logger:  logger,
		cancel:  cancel,
	}
	c.wg.Add(1)
	go c.run(runCtx)
	return c, nil
}

func (c *Consumer) run(ctx context.Context) {
	defer c.wg.Done()

	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			c.logger.Error("kafka fetch error",
				"topic", topic,
				"partition", partition,
				"err", err,
			)
		})

		fetches.EachRecord(func(rec *kgo.Record) {
			c.handle(ctx, rec)
			c.client.MarkCommitRecords(rec)
		})
	}
}

func (c *Consumer) handle(ctx context.Context, rec *kgo.Record) {
	c.metrics.EventReceived()

	var dto eventDTO
	if err := json.Unmarshal(rec.Value, &dto); err != nil {
		c.metrics.EventMalformed()
		c.logger.Warn("malformed search event",
			"topic", rec.Topic,
			"partition", rec.Partition,
			"offset", rec.Offset,
			"size", len(rec.Value),
			"err", err,
		)
		return
	}

	if err := c.ingest.Handle(ctx, apptrends.IngestEventCommand{
		Query:     dto.Query,
		Timestamp: dto.Timestamp,
	}); err != nil {
		c.logger.Debug("event rejected",
			"err", err,
			"request_id", dto.RequestID,
			"source", dto.Source,
		)
	}
}

func (c *Consumer) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		c.cancel()
		c.wg.Wait()

		commitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.client.CommitMarkedOffsets(commitCtx); err != nil {
			c.logger.Warn("kafka final commit", "err", err)
		}
		c.client.Close()
	})
}

type slogWriter struct {
	logger *slog.Logger
}

func (w slogWriter) Write(p []byte) (int, error) {
	w.logger.Debug("kafka client", "msg", string(p))
	return len(p), nil
}
