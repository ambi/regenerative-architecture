package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ra-idp-go/internal/adapters/eventsink"
	"ra-idp-go/internal/adapters/persistence/postgres"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	brokers := splitNonEmpty(os.Getenv("KAFKA_BROKERS"))
	if databaseURL == "" || len(brokers) == 0 {
		return errors.New("DATABASE_URL and KAFKA_BROKERS are required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer pool.Close()
	relay, err := eventsink.NewKafkaRelay(pool, brokers, envDefault("RELAY_CLIENT_ID", "ra-idp-go-relay"))
	if err != nil {
		return fmt.Errorf("create relay: %w", err)
	}
	defer relay.Close()
	relay.PollInterval = time.Duration(envInt("POLL_INTERVAL_MS", 200)) * time.Millisecond
	relay.BatchSize = envInt("BATCH_SIZE", 100)
	log.Printf("ra-idp-go relay started; brokers=%s", strings.Join(brokers, ","))
	if err := relay.Run(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("relay: %w", err)
	}
	return nil
}

func splitNonEmpty(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
