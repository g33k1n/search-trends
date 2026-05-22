package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr          string
	NATSURL           string
	NATSSubject       string
	Window            time.Duration
	BucketResolution  time.Duration
	MaxQueryPerBucket int64
}

func Load() Config {
	return Config{
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		NATSURL:           getenv("NATS_URL", "nats://localhost:4222"),
		NATSSubject:       getenv("NATS_SUBJECT", "search.events"),
		Window:            getDuration("WINDOW", 5*time.Minute),
		BucketResolution:  getDuration("BUCKET_RESOLUTION", time.Second),
		MaxQueryPerBucket: getInt64("MAX_QUERY_PER_BUCKET", 1000),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func getInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
