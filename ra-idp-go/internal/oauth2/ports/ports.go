// Package ports: OAuth2 ユースケースが要求する境界。
// 各 TS 個別ポートを 1 ファイルに集約（Go の慣習に合わせる）。
package ports

import (
	"context"
	"crypto"
	"time"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// クライアント / ユーザー / コンセント
// =====================================================================

type ClientRepository interface {
	FindByID(ctx context.Context, clientID string) (*spec.Client, error)
	Save(ctx context.Context, c *spec.Client) error
	Delete(ctx context.Context, clientID string) error
	FindAll(ctx context.Context) ([]*spec.Client, error)
}

type UserRepository interface {
	FindBySub(ctx context.Context, sub string) (*spec.User, error)
	FindByUsername(ctx context.Context, username string) (*spec.User, error)
	Save(ctx context.Context, user *spec.User) error
}

type ConsentRepository interface {
	Find(ctx context.Context, sub, clientID string) (*spec.Consent, error)
	Save(ctx context.Context, c *spec.Consent) error
	Revoke(ctx context.Context, sub, clientID string) error
}

// =====================================================================
// 認可リクエスト / 認可コード / PAR
// =====================================================================

type AuthorizationRequestStore interface {
	Save(ctx context.Context, req *spec.AuthorizationRequest) error
	Find(ctx context.Context, id string) (*spec.AuthorizationRequest, error)
	UpdateState(ctx context.Context, id string, state spec.AuthorizationCodeFlowState) error
	AttachSubject(ctx context.Context, id, sub string, authTime int64) error
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

// =====================================================================
// リフレッシュトークン
// =====================================================================

type RefreshTokenStore interface {
	FindByHash(ctx context.Context, hash string) (*spec.RefreshTokenRecord, error)
	Save(ctx context.Context, rec *spec.RefreshTokenRecord) error
	// Rotate は parentId を rotated にしつつ新レコードを atomic に保存。
	Rotate(ctx context.Context, parentID string, newRec *spec.RefreshTokenRecord) (*spec.RefreshTokenRecord, error)
	RevokeFamily(ctx context.Context, familyID string) error
}

// =====================================================================
// デバイスコード
// =====================================================================

type DeviceCodeStore interface {
	Save(ctx context.Context, rec *spec.DeviceAuthorization) error
	FindByDeviceCodeHash(ctx context.Context, hash string) (*spec.DeviceAuthorization, error)
	FindByUserCode(ctx context.Context, userCode string) (*spec.DeviceAuthorization, error)
	Update(ctx context.Context, rec *spec.DeviceAuthorization) error
	Exchange(ctx context.Context, deviceCodeHash string) (*spec.DeviceAuthorization, error)
}

// =====================================================================
// リプレイ防止
// =====================================================================

type DpopReplayStore interface {
	RecordIfNew(ctx context.Context, jti string, windowSeconds int, now time.Time) (bool, error)
}

type ClientAssertionReplayStore interface {
	RecordIfNew(ctx context.Context, jti string, windowSeconds int, now time.Time) (bool, error)
}

// =====================================================================
// 鍵ストア / JWT 署名・検証
// =====================================================================

// SigningKey は本実装では RSA を想定。alg=PS256 のみ。
// 公開鍵 JWK は JWKS 配布用。
type SigningKey struct {
	Kid        string
	Alg        spec.SignatureAlgorithm
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	PublicJWK  map[string]any
	Active     bool
	CreatedAt  time.Time
}

type KeyStore interface {
	GetActiveKey(ctx context.Context) (*SigningKey, error)
	GetAllKeys(ctx context.Context) ([]*SigningKey, error)
	FindByKID(ctx context.Context, kid string) (*SigningKey, error)
	Rotate(ctx context.Context) (*SigningKey, error)
}

type AccessTokenInput struct {
	Client           *spec.Client
	Sub              string
	Scopes           []string
	SenderConstraint *spec.SenderConstraint
	AuthTime         int64
}

type IDTokenInput struct {
	Client    *spec.Client
	User      *spec.User
	Scopes    []string
	Nonce     *string
	AuthTime  int64
	AtHashFor string // access token whose hash goes into at_hash
}

type TokenIssuer interface {
	SignAccessToken(ctx context.Context, in AccessTokenInput) (token, jti string, err error)
	SignIDToken(ctx context.Context, in IDTokenInput) (string, error)
	AccessTokenTTLSeconds() int
	IDTokenTTLSeconds() int
}

// IntrospectionResult は RFC 7662 のレスポンス。
type IntrospectionResult struct {
	Active           bool
	JTI              string
	ClientID         string
	Sub              string
	Scope            string
	Exp              int64
	Iat              int64
	TokenType        string
	SenderConstraint *spec.SenderConstraint
}

type TokenIntrospector interface {
	IntrospectAccessToken(ctx context.Context, token string) (*IntrospectionResult, error)
}

// =====================================================================
// セッション
// =====================================================================

type SessionStore interface {
	Save(ctx context.Context, s *spec.LoginSession) error
	Find(ctx context.Context, sessionID string) (*spec.LoginSession, error)
	Delete(ctx context.Context, sessionID string) error
}

// =====================================================================
// その他
// =====================================================================

// EventSink はドメインイベントの出力先。observable side-effect の境界。
type EventSink interface {
	Emit(ctx context.Context, e spec.DomainEvent) error
}

type Authorizer interface {
	Authorize(ctx context.Context, req spec.AuthZRequest) (spec.AuthZResponse, error)
}
