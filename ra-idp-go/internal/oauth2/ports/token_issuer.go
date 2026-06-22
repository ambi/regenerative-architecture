package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type AccessTokenInput struct {
	Client           *spec.Client
	Sub              string
	Scopes           []string
	SenderConstraint *spec.SenderConstraint
	AuthTime         int64
	AMR              []string
	ACR              string
	// AgentID は client_credentials で発行したトークンに束縛された Agent の id
	// (ADR-048)。非空のとき agent_id / principal_type=agent claim を付与する。
	AgentID string
	// Audiences は発行トークンの aud を明示指定する (RFC 8707 / RFC 8693)。
	// 空のときは従来どおり Client.ClientID を aud に用いる。len==1 なら単一文字列、
	// len>1 なら配列として書き込む。AllAccessTokensCarryAudience 不変条件のため
	// 結果の aud は常に 1 個以上になる。
	Audiences []string
	// Act は RFC 8693 §4.1 の actor claim。非 nil のとき act claim を付与する
	// (トークン交換の委任トークン用)。
	Act map[string]any
}

type IDTokenInput struct {
	Client    *spec.Client
	User      *spec.User
	Scopes    []string
	Nonce     *string
	AuthTime  int64
	AMR       []string
	ACR       string
	AtHashFor string // access token whose hash goes into at_hash
	// ResolveAttributeDefs はユーザのテナントに有効な属性定義 (builtin + custom) を
	// 返す。nil の場合は属性ベースの claim 生成をスキップする (wi-19)。
	ResolveAttributeDefs func(ctx context.Context, tenantID string) ([]spec.UserAttributeDef, error)
}

type TokenIssuer interface {
	SignAccessToken(ctx context.Context, in AccessTokenInput) (token, jti string, err error)
	SignIDToken(ctx context.Context, in IDTokenInput) (string, error)
	AccessTokenTTLSeconds() int
	IDTokenTTLSeconds() int
}
