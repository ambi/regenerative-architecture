package ports

import "context"

type EmailMessage struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type EmailSender interface {
	SendEmail(ctx context.Context, message EmailMessage) bool
}
