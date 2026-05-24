package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.KafkaTopic != "search.events" {
		t.Errorf("KafkaTopic = %q", cfg.KafkaTopic)
	}
	if cfg.KafkaGroupID != "search-trends" {
		t.Errorf("KafkaGroupID = %q", cfg.KafkaGroupID)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("KafkaBrokers = %v", cfg.KafkaBrokers)
	}
	if cfg.Window != 5*time.Minute {
		t.Errorf("Window = %s", cfg.Window)
	}
	if cfg.BucketResolution != time.Second {
		t.Errorf("BucketResolution = %s", cfg.BucketResolution)
	}
	if cfg.MaxQueryPerBucket != 1000 {
		t.Errorf("MaxQueryPerBucket = %d", cfg.MaxQueryPerBucket)
	}
}

func TestLoadKafkaBrokersCSV(t *testing.T) {
	clearEnv(t)
	t.Setenv("KAFKA_BROKERS", "broker-1:9092, broker-2:9092 ,broker-3:9092")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"broker-1:9092", "broker-2:9092", "broker-3:9092"}
	if len(cfg.KafkaBrokers) != len(want) {
		t.Fatalf("KafkaBrokers = %v, want %v", cfg.KafkaBrokers, want)
	}
	for i, b := range want {
		if cfg.KafkaBrokers[i] != b {
			t.Errorf("KafkaBrokers[%d] = %q, want %q", i, cfg.KafkaBrokers[i], b)
		}
	}
}

func TestLoadInvalidDurationFails(t *testing.T) {
	clearEnv(t)
	t.Setenv("WINDOW", "not-a-duration")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "WINDOW") {
		t.Fatalf("expected duration error for WINDOW, got %v", err)
	}
}

func TestLoadWindowLessThanResolutionFails(t *testing.T) {
	clearEnv(t)
	t.Setenv("WINDOW", "500ms")
	t.Setenv("BUCKET_RESOLUTION", "1s")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "WINDOW") {
		t.Fatalf("expected window/resolution error, got %v", err)
	}
}

func TestLoadInvalidIntegerFails(t *testing.T) {
	clearEnv(t)
	t.Setenv("MAX_QUERY_PER_BUCKET", "0")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MAX_QUERY_PER_BUCKET") {
		t.Fatalf("expected min-value error, got %v", err)
	}
}

func TestLoadIntegerAsSeconds(t *testing.T) {
	clearEnv(t)
	t.Setenv("WINDOW", "300")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Window != 300*time.Second {
		t.Errorf("Window = %s, want 5m0s", cfg.Window)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HTTP_ADDR", "KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP_ID",
		"WINDOW", "BUCKET_RESOLUTION", "MAX_QUERY_PER_BUCKET",
	} {
		t.Setenv(k, "")
	}
}
