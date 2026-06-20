package ports

import (
	"context"
	"time"
)

// EmailChangeTokenRecord は primary email 変更の再検証トークン (wi-21)。
// 新アドレスへ送るワンタイムリンクの hash と、確定時に設定する新メールを保持する。
// PasswordResetTokenRecord と同方針で平文トークンは保存しない。
type EmailChangeTokenRecord struct {
	Sub       string
	TokenHash string
	NewEmail  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type EmailChangeTokenStore interface {
	Save(ctx context.Context, record EmailChangeTokenRecord) error
	Consume(ctx context.Context, tokenHash string, now time.Time) (*EmailChangeTokenRecord, error)
}
