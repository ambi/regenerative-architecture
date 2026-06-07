// Package http: Echo v5 を用いた HTTP アダプタ。
// TS adapters/http/* に対応。
//
// 各エンドポイントは責務ごとに *_handler.go へ分割している。
// このファイルでは依存集約 (Deps) とルーティング登録 (Register) のみを定義する。
package http

import (
	"ra-idp-go/internal/adapters/crypto"
	authdomain "ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type Deps struct {
	Issuer                     string
	SCL                        *spec.SCL
	ClientRepo                 oauthports.ClientRepository
	UserRepo                   oauthports.UserRepository
	ConsentRepo                oauthports.ConsentRepository
	RequestStore               oauthports.AuthorizationRequestStore
	CodeStore                  oauthports.AuthorizationCodeStore
	PARStore                   oauthports.PARStore
	RefreshStore               oauthports.RefreshTokenStore
	DeviceCodeStore            oauthports.DeviceCodeStore
	DpopReplayStore            oauthports.DpopReplayStore
	ClientAssertionReplayStore oauthports.ClientAssertionReplayStore
	KeyStore                   oauthports.KeyStore
	TokenIssuer                oauthports.TokenIssuer
	TokenIntrospector          oauthports.TokenIntrospector
	Authorizer                 oauthports.Authorizer
	JWKResolver                *crypto.JWKResolver
	PasswordHasher             authports.PasswordHasher
	SessionManager             *authusecases.SessionManager
	AuthnResolver              authdomain.AuthenticationContextResolver
	Emit                       func(spec.DomainEvent)
	HealthInfo                 HealthInfo
}

func Register(e *echo.Echo, d Deps) {
	e.GET("/authorize", d.handleAuthorize)
	e.GET("/end_session", d.handleEndSession)
	e.POST("/end_session", d.handleEndSession)
	e.GET("/api/auth/transaction", d.handleTransaction)
	e.POST("/api/auth/login", d.handleLoginAPI)
	e.POST("/api/auth/consent", d.handleConsentAPI)
	e.GET("/api/auth/device", d.handleDeviceContext)
	e.POST("/api/auth/device", d.handleDeviceAPI)
	e.POST("/token", d.handleToken)
	e.POST("/revoke", d.handleRevoke)
	e.POST("/introspect", d.handleIntrospect)
	e.GET("/userinfo", d.handleUserInfo)
	e.POST("/userinfo", d.handleUserInfo)
	e.POST("/register", d.handleRegisterClient)
	e.POST("/par", d.handlePAR)
	e.POST("/device_authorization", d.handleDeviceAuthorization)
	e.GET("/.well-known/openid-configuration", d.handleDiscovery)
	e.GET("/jwks", d.handleJWKS)
	e.GET("/health", d.handleHealth)
}
