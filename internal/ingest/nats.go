package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

type SearchEvent struct {
	Query     string    `json:"query"`
	UserID    string    `json:"user_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source,omitempty"`
}

type Store interface {
	Add(query string, at time.Time) bool
}

type Metrics struct {
	Received  atomic.Uint64
	Accepted  atomic.Uint64
	Rejected  atomic.Uint64
	Malformed atomic.Uint64
}

type Consumer struct {
	conn    *nats.Conn
	sub     *nats.Subscription
	metrics *Metrics
}

func StartNATS(ctx context.Context, url, subject string, store Store, metrics *Metrics, logger *slog.Logger) (*Consumer, error) {
	if store == nil {
		return nil, errors.New("store is nil")
	}
	if metrics == nil {
		metrics = &Metrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}

	conn, err := nats.Connect(
		url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, err
	}

	sub, err := conn.Subscribe(subject, func(msg *nats.Msg) {
		metrics.Received.Add(1)

		var event SearchEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			metrics.Malformed.Add(1)
			logger.Warn("malformed search event", "error", err)
			return
		}

		event.Query = strings.TrimSpace(event.Query)
		if event.Query == "" {
			metrics.Rejected.Add(1)
			return
		}

		if store.Add(event.Query, event.Timestamp) {
			metrics.Accepted.Add(1)
			return
		}
		metrics.Rejected.Add(1)
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	consumer := &Consumer{conn: conn, sub: sub, metrics: metrics}
	go func() {
		<-ctx.Done()
		consumer.Close()
	}()

	return consumer, nil
}

func (c *Consumer) Close() {
	if c == nil {
		return
	}
	if c.sub != nil {
		_ = c.sub.Drain()
	}
	if c.conn != nil {
		c.conn.Drain()
		c.conn.Close()
	}
}
