package notification

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

// SMTPTLSMode は SMTP 接続時の TLS 戦略を表す (ADR-035 §2)。
type SMTPTLSMode string

const (
	SMTPTLSSTARTTLS SMTPTLSMode = "starttls"
	SMTPTLSImplicit SMTPTLSMode = "implicit"
	SMTPTLSNone     SMTPTLSMode = "none"
)

type SMTPEmailSenderConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Hello    string
	TLSMode  SMTPTLSMode
	Timeout  time.Duration
}

type SMTPEmailSender struct {
	config SMTPEmailSenderConfig
}

func NewSMTPEmailSender(cfg SMTPEmailSenderConfig) *SMTPEmailSender {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Hello == "" {
		cfg.Hello = "localhost"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = SMTPTLSSTARTTLS
	}
	return &SMTPEmailSender{config: cfg}
}

func (s *SMTPEmailSender) SendEmail(ctx context.Context, message authports.EmailMessage) bool {
	if err := s.send(ctx, message, time.Now().UTC()); err != nil {
		log.Printf("smtp send failed: to=%s subject=%q err=%v", message.To, message.Subject, err)
		return false
	}
	return true
}

func (s *SMTPEmailSender) send(ctx context.Context, message authports.EmailMessage, now time.Time) error {
	addr := net.JoinHostPort(s.config.Host, fmt.Sprintf("%d", s.config.Port))
	dialer := &net.Dialer{Timeout: s.config.Timeout}

	var conn net.Conn
	var err error
	if s.config.TLSMode == SMTPTLSImplicit {
		tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsConfig}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	deadline := now.Add(s.config.Timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Hello(s.config.Hello); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}

	if s.config.TLSMode == SMTPTLSSTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not advertise STARTTLS")
		}
		tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.config.Username != "" {
		if s.config.TLSMode == SMTPTLSNone {
			return errors.New("smtp PLAIN auth requires TLS; set SMTP_TLS to implicit or starttls")
		}
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	body, err := buildRFC5322Message(s.config.From, message, now)
	if err != nil {
		_ = writer.Close()
		return err
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

// buildRFC5322Message は EmailMessage を RFC 5322 形式の本文に変換する。
// Text/HTML 両方ある場合は multipart/alternative (ADR-035 §8)。
func buildRFC5322Message(from string, message authports.EmailMessage, now time.Time) (string, error) {
	messageID, err := newMessageID(from, now)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", message.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", message.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", messageID)
	b.WriteString("MIME-Version: 1.0\r\n")

	hasText := message.Text != ""
	hasHTML := message.HTML != ""
	switch {
	case hasText && hasHTML:
		boundary, err := randomBoundary()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
		writePart(&b, boundary, "text/plain; charset=utf-8", message.Text)
		writePart(&b, boundary, "text/html; charset=utf-8", message.HTML)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	case hasHTML:
		b.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(message.HTML)
		b.WriteString("\r\n")
	default:
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(message.Text)
		b.WriteString("\r\n")
	}
	return b.String(), nil
}

func writePart(b *strings.Builder, boundary, contentType, body string) {
	fmt.Fprintf(b, "--%s\r\n", boundary)
	fmt.Fprintf(b, "Content-Type: %s\r\n", contentType)
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
}

func randomBoundary() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("boundary entropy: %w", err)
	}
	return "ra-idp-" + hex.EncodeToString(buf), nil
}

func newMessageID(from string, now time.Time) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("message-id entropy: %w", err)
	}
	domain := domainOf(from)
	return fmt.Sprintf("<%d.%s@%s>", now.UnixNano(), hex.EncodeToString(buf), domain), nil
}

func domainOf(address string) string {
	if i := strings.LastIndex(address, "@"); i >= 0 && i+1 < len(address) {
		return address[i+1:]
	}
	return "localhost"
}
