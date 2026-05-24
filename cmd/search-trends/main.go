package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/g33k1n/search-trends/internal/adapter/http"
	kafkaadapter "github.com/g33k1n/search-trends/internal/adapter/kafka"
	apptrends "github.com/g33k1n/search-trends/internal/application/trends"
	"github.com/g33k1n/search-trends/internal/config"
	"github.com/g33k1n/search-trends/internal/infra/metrics"
	infratrends "github.com/g33k1n/search-trends/internal/infra/trends"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	store := infratrends.NewMemoryStore(cfg.Window, cfg.BucketResolution, cfg.MaxQueryPerBucket)
	sink := metrics.NewSink()

	ingestUC := apptrends.NewIngestEventUseCase(store, sink)
	queryUC := apptrends.NewQueryTopUseCase(store)
	stopListUC := apptrends.NewStopListUseCase(store)

	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	consumer, err := kafkaadapter.Start(rootCtx, kafkaadapter.Config{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTopic,
		GroupID: cfg.KafkaGroupID,
	}, ingestUC, sink, logger)
	if err != nil {
		logger.Error("start kafka consumer", "err", err)
		os.Exit(1)
	}

	apiServer := httpadapter.NewServer(queryUC, stopListUC, sink.HTTPHandler())
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			logger.Error("http server failed", "err", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown", "err", err)
	}
	consumer.Close()

	logger.Info("shutdown complete")
}
