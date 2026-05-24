package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr          string
	KafkaBrokers      []string
	KafkaTopic        string
	KafkaGroupID      string
	Window            time.Duration
	BucketResolution  time.Duration
	MaxQueryPerBucket int64
}

func Load() (Config, error) {
	window, err := getDuration("WINDOW", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	resolution, err := getDuration("BUCKET_RESOLUTION", time.Second)
	if err != nil {
		return Config{}, err
	}
	maxPerBucket, err := getInt64("MAX_QUERY_PER_BUCKET", 1000)
	if err != nil {
		return Config{}, err
	}

	if window <= 0 {
		return Config{}, fmt.Errorf("WINDOW must be positive, got %s", window)
	}
	if resolution <= 0 {
		return Config{}, fmt.Errorf("BUCKET_RESOLUTION must be positive, got %s", resolution)
	}
	if window < resolution {
		return Config{}, fmt.Errorf("WINDOW (%s) must be >= BUCKET_RESOLUTION (%s)", window, resolution)
	}

	brokers := splitCSV(getenv("KAFKA_BROKERS", "localhost:9092"))
	if len(brokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS must not be empty")
	}

	return Config{
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		KafkaBrokers:      brokers,
		KafkaTopic:        getenv("KAFKA_TOPIC", "search.events"),
		KafkaGroupID:      getenv("KAFKA_GROUP_ID", "search-trends"),
		Window:            window,
		BucketResolution:  resolution,
		MaxQueryPerBucket: maxPerBucket,
	}, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return 0, fmt.Errorf("invalid duration for %s: %q", key, value)
}

func getInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %q", key, value)
	}
	if parsed < 1 {
		return 0, fmt.Errorf("%s must be >= 1, got %d", key, parsed)
	}
	return parsed, nil
}
