package bootstrap

import (
	"fmt"
	"log"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/platform/notification"
)

// resolveEmailSender は EMAIL_SENDER / SMTP_* 環境変数から EmailSender adapter を組み立てる。
// 既定は console。smtp 選択時に SMTP_HOST / SMTP_FROM が無い場合は起動失敗 (ADR-035 §影響)。
func resolveEmailSender(getenv func(string) string) (authports.EmailSender, error) {
	kind := strings.ToLower(strings.TrimSpace(getenv("EMAIL_SENDER")))
	if kind == "" {
		kind = "console"
	}
	switch kind {
	case "console":
		log.Printf("email sender: console (dev / demo)")
		return notification.ConsoleEmailSender{}, nil
	case "smtp":
		cfg, err := buildSMTPConfig(getenv)
		if err != nil {
			return nil, err
		}
		log.Printf("email sender: smtp host=%s port=%d tls=%s from=%s",
			cfg.Host, cfg.Port, cfg.TLSMode, cfg.From)
		return notification.NewSMTPEmailSender(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported EMAIL_SENDER=%q (want console or smtp)", kind)
	}
}

func buildSMTPConfig(getenv func(string) string) (notification.SMTPEmailSenderConfig, error) {
	var zero notification.SMTPEmailSenderConfig
	host := strings.TrimSpace(getenv("SMTP_HOST"))
	if host == "" {
		return zero, fmt.Errorf("EMAIL_SENDER=smtp requires SMTP_HOST")
	}
	from := strings.TrimSpace(getenv("SMTP_FROM"))
	if from == "" {
		return zero, fmt.Errorf("EMAIL_SENDER=smtp requires SMTP_FROM")
	}
	tlsMode, err := parseSMTPTLSMode(getenv("SMTP_TLS"))
	if err != nil {
		return zero, err
	}
	return notification.SMTPEmailSenderConfig{
		Host:     host,
		Port:     parseSMTPPort(getenv("SMTP_PORT"), tlsMode),
		Username: getenv("SMTP_USERNAME"),
		Password: getenv("SMTP_PASSWORD"),
		From:     from,
		Hello:    strings.TrimSpace(getenv("SMTP_HELO")),
		TLSMode:  tlsMode,
		Timeout:  parseSMTPTimeout(getenv("SMTP_TIMEOUT_SECONDS")),
	}, nil
}

func parseSMTPTLSMode(raw string) (notification.SMTPTLSMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "starttls":
		return notification.SMTPTLSSTARTTLS, nil
	case "implicit":
		return notification.SMTPTLSImplicit, nil
	case "none":
		return notification.SMTPTLSNone, nil
	default:
		return "", fmt.Errorf("unsupported SMTP_TLS=%q (want starttls, implicit, or none)", raw)
	}
}

func parseSMTPPort(raw string, mode notification.SMTPTLSMode) int {
	if port := envIntFrom(raw, 0); port > 0 {
		return port
	}
	switch mode {
	case notification.SMTPTLSImplicit:
		return 465
	case notification.SMTPTLSNone:
		return 25
	default:
		return 587
	}
}

func parseSMTPTimeout(raw string) time.Duration {
	seconds := envIntFrom(raw, 10)
	if seconds <= 0 {
		seconds = 10
	}
	return time.Duration(seconds) * time.Second
}

func envIntFrom(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}
