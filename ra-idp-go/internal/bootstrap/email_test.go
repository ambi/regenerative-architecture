package bootstrap

import (
	"strings"
	"testing"

	"ra-idp-go/internal/shared/adapters/notification"
)

func TestResolveEmailSenderDefaultsToConsole(t *testing.T) {
	t.Parallel()
	sender, err := resolveEmailSender(stubEnv(map[string]string{}))
	if err != nil {
		t.Fatalf("resolveEmailSender: %v", err)
	}
	if _, ok := sender.(notification.ConsoleEmailSender); !ok {
		t.Fatalf("default sender = %T, want ConsoleEmailSender", sender)
	}
}

func TestResolveEmailSenderSMTPRequiresHostAndFrom(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"missing host", map[string]string{"EMAIL_SENDER": "smtp", "SMTP_FROM": "a@b"}, "SMTP_HOST"},
		{"missing from", map[string]string{"EMAIL_SENDER": "smtp", "SMTP_HOST": "h"}, "SMTP_FROM"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveEmailSender(stubEnv(tc.env))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestResolveEmailSenderSMTPBuildsAdapter(t *testing.T) {
	t.Parallel()
	sender, err := resolveEmailSender(stubEnv(map[string]string{
		"EMAIL_SENDER":  "smtp",
		"SMTP_HOST":     "smtp.example.com",
		"SMTP_FROM":     "noreply@example.com",
		"SMTP_USERNAME": "apikey",
		"SMTP_PASSWORD": "s3cret",
	}))
	if err != nil {
		t.Fatalf("resolveEmailSender: %v", err)
	}
	if _, ok := sender.(*notification.SMTPEmailSender); !ok {
		t.Fatalf("smtp sender = %T, want *SMTPEmailSender", sender)
	}
}

func TestResolveEmailSenderRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	_, err := resolveEmailSender(stubEnv(map[string]string{"EMAIL_SENDER": "carrier-pigeon"}))
	if err == nil || !strings.Contains(err.Error(), "EMAIL_SENDER") {
		t.Fatalf("err=%v, want unsupported EMAIL_SENDER error", err)
	}
}

func TestParseSMTPTLSModeDefault(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "starttls", "STARTTLS"} {
		mode, err := parseSMTPTLSMode(raw)
		if err != nil {
			t.Fatalf("parseSMTPTLSMode(%q): %v", raw, err)
		}
		if mode != notification.SMTPTLSSTARTTLS {
			t.Fatalf("parseSMTPTLSMode(%q) = %q, want starttls", raw, mode)
		}
	}
}

func TestParseSMTPPortDefaultsPerMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode notification.SMTPTLSMode
		want int
	}{
		{notification.SMTPTLSSTARTTLS, 587},
		{notification.SMTPTLSImplicit, 465},
		{notification.SMTPTLSNone, 25},
	}
	for _, tc := range cases {
		if got := parseSMTPPort("", tc.mode); got != tc.want {
			t.Errorf("mode=%s port=%d, want %d", tc.mode, got, tc.want)
		}
	}
	if got := parseSMTPPort("2525", notification.SMTPTLSSTARTTLS); got != 2525 {
		t.Errorf("explicit port override = %d, want 2525", got)
	}
}

func stubEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
