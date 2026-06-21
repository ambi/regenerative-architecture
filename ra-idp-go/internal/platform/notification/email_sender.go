package notification

import (
	"context"
	"log"

	authports "ra-idp-go/internal/authentication/ports"
)

type ConsoleEmailSender struct{}

func (ConsoleEmailSender) SendEmail(_ context.Context, message authports.EmailMessage) bool {
	log.Printf("email to=%s subject=%q\n%s", message.To, message.Subject, message.Text)
	return true
}

type NoopEmailSender struct {
	Sent []authports.EmailMessage
}

func (s *NoopEmailSender) SendEmail(_ context.Context, message authports.EmailMessage) bool {
	s.Sent = append(s.Sent, message)
	return true
}
