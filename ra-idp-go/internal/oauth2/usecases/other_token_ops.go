// /revoke, /userinfo, register_client, push_authorization_request, rotate_signing_key 等の
// 残りのユースケース。
package usecases

import (
	"context"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

// =====================================================================
// /revoke (RFC 7009)
// =====================================================================

type RevokeDeps struct {
	RefreshStore ports.RefreshTokenStore
	Emit         func(spec.DomainEvent)
}

func RevokeToken(ctx context.Context, deps RevokeDeps, token string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	hash := domain.HashRefreshToken(token)
	rec, err := deps.RefreshStore.FindByHash(ctx, hash)
	if err != nil {
		return err
	}
	if rec == nil {
		// RFC 7009 §2.2: 未知トークンは 200 OK no-op
		return nil
	}
	if err := deps.RefreshStore.RevokeFamily(ctx, rec.FamilyID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.TokenRevoked{At: now, TokenType: "refresh_token", TokenID: rec.ID, Reason: "client_initiated"})
	return nil
}

// =====================================================================
// /userinfo (OIDC Core §5.3)
// =====================================================================

type UserInfoInput struct {
	Scopes   []string
	Sub      string
	Active   bool
	ClientID string
}

type UserInfoResponse struct {
	Sub               string `json:"sub"`
	Name              string `json:"name,omitempty"`
	GivenName         string `json:"given_name,omitempty"`
	FamilyName        string `json:"family_name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	UpdatedAt         int64  `json:"updated_at,omitempty"`
}

func UserInfo(
	ctx context.Context,
	repo ports.UserRepository,
	authorizer ports.Authorizer,
	in UserInfoInput,
) (*UserInfoResponse, error) {
	req := spec.AuthZRequest{
		Subject:  spec.AuthZSubject{Type: "Client", ID: in.ClientID},
		Action:   spec.ActionUserInfoRead,
		Resource: spec.AuthZResource{Type: "UserInfo", Properties: spec.AuthZResourceProps{Scopes: in.Scopes, Revoked: !in.Active}},
	}
	d := spec.Evaluate(req)
	if authorizer != nil {
		var err error
		d, err = authorizer.Authorize(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	if !d.Permit {
		if slices.Contains(d.Reasons, "token_has_openid_scope") {
			return nil, NewOAuthError("insufficient_scope", "openid スコープが必要")
		}
		return nil, NewOAuthError("invalid_request", "userinfo 拒否: "+strings.Join(d.Reasons, ", "))
	}
	u, err := repo.FindBySub(ctx, in.Sub)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, NewOAuthError("invalid_request", "ユーザーが存在しません")
	}
	res := &UserInfoResponse{Sub: u.Sub}
	if slices.Contains(in.Scopes, "profile") {
		if u.Name != nil {
			res.Name = *u.Name
		}
		if u.GivenName != nil {
			res.GivenName = *u.GivenName
		}
		if u.FamilyName != nil {
			res.FamilyName = *u.FamilyName
		}
		res.PreferredUsername = u.PreferredUsername
		res.UpdatedAt = u.UpdatedAt.Unix()
	}
	if slices.Contains(in.Scopes, "email") && u.Email != nil {
		res.Email = *u.Email
		res.EmailVerified = u.EmailVerified
	}
	return res, nil
}

// =====================================================================
// /register (RFC 7591 Dynamic Client Registration)
// =====================================================================

type RegisterClientInput struct {
	ClientName              string
	ClientType              spec.ClientType
	RedirectURIs            []string
	GrantTypes              []spec.GrantType
	ResponseTypes           []spec.ResponseType
	TokenEndpointAuthMethod spec.TokenEndpointAuthMethod
	Scope                   string
	JWKS                    map[string]any
	JwksURI                 *string
	RequirePAR              bool
	DpopBoundAccessTokens   bool
	FapiProfile             spec.FapiProfile
}

type RegisterClientResult struct {
	Client       *spec.Client
	ClientSecret string // 平文。出力後は再表示されない (RFC 7591 §3.2.1)
}

type RegisterClientDeps struct {
	ClientRepo ports.ClientRepository
	Emit       func(spec.DomainEvent)
}

func RegisterClient(ctx context.Context, deps RegisterClientDeps, in RegisterClientInput, now time.Time) (*RegisterClientResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if in.ClientType == "" {
		in.ClientType = spec.ClientConfidential
	}
	if len(in.RedirectURIs) == 0 {
		return nil, NewOAuthError("invalid_redirect_uri", "redirect_uris is required")
	}
	if len(in.GrantTypes) == 0 {
		in.GrantTypes = []spec.GrantType{spec.GrantAuthorizationCode}
	}
	if len(in.ResponseTypes) == 0 {
		in.ResponseTypes = []spec.ResponseType{spec.ResponseTypeCode}
	}
	if in.TokenEndpointAuthMethod == "" {
		if in.ClientType == spec.ClientPublic {
			in.TokenEndpointAuthMethod = spec.AuthMethodNone
		} else {
			in.TokenEndpointAuthMethod = spec.AuthMethodClientSecretBasic
		}
	}
	if in.TokenEndpointAuthMethod == spec.AuthMethodPrivateKeyJwt {
		candidate := spec.Client{
			ClientID: "validation", ClientType: in.ClientType,
			RedirectURIs:             []string{"https://validation.invalid/callback"},
			GrantTypes:               []spec.GrantType{spec.GrantClientCredentials},
			TokenEndpointAuthMethod:  spec.AuthMethodPrivateKeyJwt,
			JWKS:                     in.JWKS,
			JwksURI:                  in.JwksURI,
			IDTokenSignedResponseAlg: spec.SigAlgPS256,
			FapiProfile:              spec.FapiNone,
			CreatedAt:                now,
		}
		if err := candidate.Validate(); err != nil {
			return nil, NewOAuthError("invalid_client_metadata", "private_key_jwt requires non-empty inline jwks")
		}
	}
	id, err := generateOpaqueToken(16)
	if err != nil {
		return nil, err
	}
	clientID := "c_" + id
	var secret string
	var secretHash *string
	usesSecret := in.TokenEndpointAuthMethod == spec.AuthMethodClientSecretBasic ||
		in.TokenEndpointAuthMethod == spec.AuthMethodClientSecretPost
	if in.ClientType == spec.ClientConfidential && usesSecret {
		s, err := generateOpaqueToken(32)
		if err != nil {
			return nil, err
		}
		secret = s
		ss := domain.HashClientSecret(s)
		secretHash = &ss
	}
	fapiProfile := in.FapiProfile
	if fapiProfile == "" {
		fapiProfile = spec.FapiNone
	}
	scope := in.Scope
	if scope == "" {
		scope = "openid profile email"
	}
	c := &spec.Client{
		ClientID:                           clientID,
		ClientSecretHash:                   secretHash,
		ClientType:                         in.ClientType,
		RedirectURIs:                       in.RedirectURIs,
		GrantTypes:                         in.GrantTypes,
		ResponseTypes:                      in.ResponseTypes,
		TokenEndpointAuthMethod:            in.TokenEndpointAuthMethod,
		Scope:                              scope,
		JWKS:                               in.JWKS,
		JwksURI:                            in.JwksURI,
		IDTokenSignedResponseAlg:           spec.SigAlgPS256,
		RequirePushedAuthorizationRequests: in.RequirePAR,
		DpopBoundAccessTokens:              in.DpopBoundAccessTokens,
		FapiProfile:                        fapiProfile,
		CreatedAt:                          now,
	}
	if in.ClientName != "" {
		name := in.ClientName
		c.ClientName = &name
	}
	if err := c.Validate(); err != nil {
		return nil, NewOAuthError("invalid_client_metadata", err.Error())
	}
	if err := deps.ClientRepo.Save(ctx, c); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ClientRegistered{At: now, ClientID: clientID, ClientType: in.ClientType})
	return &RegisterClientResult{Client: c, ClientSecret: secret}, nil
}

// =====================================================================
// /par (RFC 9126 Pushed Authorization Request)
// =====================================================================

type PARInput struct {
	ClientID   string
	Parameters map[string]string
}

type PARResult struct {
	RequestURI string
	ExpiresIn  int
}

type PARDeps struct {
	ClientRepo ports.ClientRepository
	Store      ports.PARStore
	Emit       func(spec.DomainEvent)
}

func PushAuthorizationRequest(ctx context.Context, deps PARDeps, in PARInput, now time.Time) (*PARResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	client, err := deps.ClientRepo.FindByID(ctx, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "未知の client_id")
	}
	id, err := generateOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	requestURI := "urn:ietf:params:oauth:request_uri:" + id
	rec := &spec.PARRecord{
		RequestURI: requestURI,
		ClientID:   in.ClientID,
		Parameters: in.Parameters,
		IssuedAt:   now,
		ExpiresAt:  now.Add(90 * time.Second), // RFC 9126 §4 推奨上限
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	if err := deps.Store.Save(ctx, rec); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.PARStored{At: now, RequestURI: requestURI, ClientID: in.ClientID})
	return &PARResult{RequestURI: requestURI, ExpiresIn: 90}, nil
}

// =====================================================================
// /rotate (内部運用 — JWKS 鍵回転)
// =====================================================================

type RotateSigningKeyDeps struct {
	KeyStore ports.KeyStore
	Emit     func(spec.DomainEvent)
}

func RotateSigningKey(ctx context.Context, deps RotateSigningKeyDeps, now time.Time) (*ports.SigningKey, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	prev, _ := deps.KeyStore.GetActiveKey(ctx)
	next, err := deps.KeyStore.Rotate(ctx)
	if err != nil {
		return nil, err
	}
	prevKID := ""
	if prev != nil {
		prevKID = prev.Kid
	}
	emit(deps.Emit, &spec.SigningKeyRotated{At: now, NewKID: next.Kid, PreviousKID: prevKID})
	return next, nil
}
