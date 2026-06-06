package relay

import (
	"errors"
	"os"
	"time"
)

// Config は ra-idp-relay の起動構成。すべて環境変数から組み立てる。
type Config struct {
	DatabaseURL  string
	Brokers      []string
	ClientID     string
	PollInterval time.Duration
	BatchSize    int
}

func loadConfig() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	brokers := splitNonEmpty(os.Getenv("KAFKA_BROKERS"))
	if databaseURL == "" || len(brokers) == 0 {
		return Config{}, errors.New("DATABASE_URL and KAFKA_BROKERS are required")
	}
	return Config{
		DatabaseURL:  databaseURL,
		Brokers:      brokers,
		ClientID:     envDefault("RELAY_CLIENT_ID", "ra-idp-go-relay"),
		PollInterval: time.Duration(envInt("POLL_INTERVAL_MS", 200)) * time.Millisecond,
		BatchSize:    envInt("BATCH_SIZE", 100),
	}, nil
}
