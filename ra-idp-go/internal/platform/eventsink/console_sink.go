// ConsoleSink は Console を oauth2/ports.EventSink interface に適合させるアダプタ。
// Console.Emit は ctx を取らないため、ここで shim する。
package eventsink

import (
	"context"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

type ConsoleSink struct {
	console *Console
}

func NewConsoleSink() oauthports.EventSink {
	return &ConsoleSink{console: NewConsole()}
}

func (s *ConsoleSink) Emit(_ context.Context, event spec.DomainEvent) error {
	s.console.Emit(event)
	return nil
}
