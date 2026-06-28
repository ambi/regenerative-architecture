// Package eventsink: ドメインイベントの出力先アダプタ。
// TS adapters/event-sink/console.ts に対応 (本実装は JSON 出力)。
package eventsink

import (
	"io"
	"log"
	"os"
	"sync"

	"ra-idp-go/internal/spec"
)

type Console struct {
	mu  sync.Mutex
	out io.Writer
}

func NewConsole() *Console {
	return &Console{out: os.Stdout}
}

func (c *Console) Emit(e spec.DomainEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := spec.MarshalDomainEvent(e)
	if err != nil {
		log.Printf("event encode: %v", err)
		return
	}
	_, _ = c.out.Write(append(b, '\n'))
}
