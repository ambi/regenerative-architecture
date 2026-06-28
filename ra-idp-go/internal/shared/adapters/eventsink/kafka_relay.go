package eventsink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaRelay struct {
	Pool         *pgxpool.Pool
	Kafka        *kgo.Client
	PollInterval time.Duration
	BatchSize    int
}

type outboxRecord struct {
	ID        int64
	EventType string
	Topic     string
	Payload   []byte
}

func NewKafkaRelay(pool *pgxpool.Pool, brokers []string, clientID string) (*KafkaRelay, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	)
	if err != nil {
		return nil, err
	}
	return &KafkaRelay{
		Pool: pool, Kafka: client, PollInterval: 200 * time.Millisecond, BatchSize: 100,
	}, nil
}

func (r *KafkaRelay) Close() { r.Kafka.Close() }

func (r *KafkaRelay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.PollInterval)
	defer ticker.Stop()
	for {
		if err := r.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *KafkaRelay) Tick(ctx context.Context) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT id,event_type,topic,payload FROM outbox
WHERE published_at IS NULL ORDER BY id FOR UPDATE SKIP LOCKED LIMIT $1`, r.BatchSize)
	if err != nil {
		return err
	}
	var batch []outboxRecord
	for rows.Next() {
		var rec outboxRecord
		if err := rows.Scan(&rec.ID, &rec.EventType, &rec.Topic, &rec.Payload); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, rec)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(batch) == 0 {
		return tx.Commit(ctx)
	}

	var published []int64
	for _, rec := range batch {
		record := &kgo.Record{
			Topic: rec.Topic,
			Key:   []byte(partitionKey(rec.EventType, rec.Payload)),
			Value: rec.Payload,
			Headers: []kgo.RecordHeader{
				{Key: "event_type", Value: []byte(rec.EventType)},
				{Key: "outbox_id", Value: []byte(strconv.FormatInt(rec.ID, 10))},
			},
		}
		if err := r.Kafka.ProduceSync(ctx, record).FirstErr(); err != nil {
			if _, updateErr := tx.Exec(ctx, `UPDATE outbox SET attempts=attempts+1,last_error=$1 WHERE id=$2`,
				truncate(err.Error(), 500), rec.ID); updateErr != nil {
				return updateErr
			}
			continue
		}
		published = append(published, rec.ID)
	}
	if len(published) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at=now(),published_to='kafka',
attempts=attempts+1,last_error=NULL WHERE id=ANY($1)`, published); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func partitionKey(eventType string, payload []byte) string {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return ""
	}
	fields := map[string][]string{
		"ClientRegistered":             {"ClientID", "clientId"},
		"AdminOAuth2ClientCreated":     {"ClientID", "clientId"},
		"AdminOAuth2ClientUpdated":     {"ClientID", "clientId"},
		"AdminOAuth2ClientDeleted":     {"ClientID", "clientId"},
		"UserAuthenticated":            {"Sub", "sub"},
		"AuthenticationFailed":         {"Username", "username"},
		"LoginThrottled":               {"KeyHash", "keyHash"},
		"PasswordChanged":              {"Sub", "sub"},
		"ConsentGranted":               {"Sub", "sub"},
		"ConsentRevoked":               {"Sub", "sub"},
		"AuthorizationCodeIssued":      {"Sub", "sub"},
		"AuthorizationCodeRedeemed":    {"Sub", "sub"},
		"AccessTokenIssued":            {"Sub", "sub"},
		"RefreshTokenIssued":           {"FamilyID", "familyId"},
		"RefreshTokenRotated":          {"FamilyID", "familyId"},
		"TokenRevoked":                 {"TokenID", "tokenId"},
		"TokenIntrospected":            {"RSClientID", "rsClientId"},
		"TokenExchanged":               {"SubjectSub", "subjectSub"},
		"TokenExchangeRejected":        {"ActorSub", "actorSub"},
		"RefreshTokenReuseDetected":    {"FamilyID", "familyId"},
		"SigningKeyRotated":            {"NewKID", "newKid"},
		"PARStored":                    {"ClientID", "clientId"},
		"DeviceAuthorizationRequested": {"ClientID", "clientId"},
		"DeviceAuthorizationApproved":  {"ClientID", "clientId"},
		"DeviceAuthorizationDenied":    {"ClientID", "clientId"},
		"AgentRegistered":              {"AgentID", "agentId"},
		"AgentUpdated":                 {"AgentID", "agentId"},
		"AgentDisabled":                {"AgentID", "agentId"},
		"AgentEnabled":                 {"AgentID", "agentId"},
		"AgentKilled":                  {"AgentID", "agentId"},
		"AgentDeleted":                 {"AgentID", "agentId"},
		"AgentOwnerChanged":            {"AgentID", "agentId"},
		"AgentCredentialBound":         {"AgentID", "agentId"},
		"AgentCredentialUnbound":       {"AgentID", "agentId"},
	}
	for _, field := range fields[eventType] {
		if v, ok := value[field]; ok {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
