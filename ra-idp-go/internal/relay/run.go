package relay

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"ra-idp-go/internal/shared/adapters/eventsink"
	"ra-idp-go/internal/shared/adapters/persistence/postgres"
)

// Run は outbox → Kafka リレーを起動する。SIGINT/SIGTERM で graceful shutdown。
func Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer pool.Close()
	relay, err := eventsink.NewKafkaRelay(pool, cfg.Brokers, cfg.ClientID)
	if err != nil {
		return fmt.Errorf("create relay: %w", err)
	}
	defer relay.Close()
	relay.PollInterval = cfg.PollInterval
	relay.BatchSize = cfg.BatchSize
	log.Printf("ra-idp-go relay started; brokers=%s", strings.Join(cfg.Brokers, ","))
	if err := relay.Run(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("relay: %w", err)
	}
	return nil
}
