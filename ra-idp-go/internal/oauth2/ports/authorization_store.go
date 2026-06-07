package ports

import (
	"context"
	"time"

	"ra-idp-go/internal/spec"
)

type AuthorizationRequestStore interface {
	Save(ctx context.Context, req *spec.AuthorizationRequest) error
	Find(ctx context.Context, id string) (*spec.AuthorizationRequest, error)
	UpdateState(ctx context.Context, id string, state spec.AuthorizationCodeFlowState) error
	AttachAuthentication(ctx context.Context, id, sub string, authTime int64, amr []string, acr string) error
}

type AuthorizationCodeStore interface {
	Save(ctx context.Context, code *spec.AuthorizationCodeRecord) error
	Find(ctx context.Context, code string) (*spec.AuthorizationCodeRecord, error)
	// Redeem は code を atomic に redeemed にする。既に redeemed なら nil。
	Redeem(ctx context.Context, code string, now time.Time) (*spec.AuthorizationCodeRecord, error)
	// LinkFamily は成功交換時の refresh family を逆引きインデックスに紐付ける。
	LinkFamily(ctx context.Context, code, familyID string) error
}

type PARStore interface {
	Save(ctx context.Context, rec *spec.PARRecord) error
	Find(ctx context.Context, requestURI string) (*spec.PARRecord, error)
	Consume(ctx context.Context, requestURI string) (*spec.PARRecord, error)
}
