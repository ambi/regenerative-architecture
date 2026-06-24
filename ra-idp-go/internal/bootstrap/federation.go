package bootstrap

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"ra-idp-go/internal/federation/adapters/samltoken"
	federationports "ra-idp-go/internal/federation/ports"
	"ra-idp-go/internal/spec"
)

// newDevFederationSigner は開発用の自己署名 federation 署名証明書から署名器を作る。
// 本番の証明書ライフサイクル・ローテーション・metadata 掲載は後続スライス (ADR-060) で扱う。
func newDevFederationSigner() (*samltoken.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate federation signing key: %w", err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "ra-idp federation signing (dev)"},
		NotBefore:    now.Add(-1 * time.Hour),
		NotAfter:     now.Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create federation signing certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse federation signing certificate: %w", err)
	}
	return samltoken.NewSigner(cert, key)
}

// seedWsFedRelyingParty は WS-Federation passive のデモ用 relying party を投入する。
func seedWsFedRelyingParty(ctx context.Context, repo federationports.WsFedRelyingPartyRepository) error {
	now := time.Now().UTC()
	rp := &spec.WsFedRelyingParty{
		TenantID:    spec.DefaultTenantID,
		Wtrealm:     "urn:ra-idp:demo-rp",
		DisplayName: "Demo WS-Federation RP",
		ReplyURLs:   []string{"https://rp.example/wsfed"},
		ClaimPolicy: spec.ClaimMappingPolicy{
			NameID: spec.NameIdConfiguration{
				Format:          "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
				SourceAttribute: "sub",
			},
			Rules: []spec.ClaimMappingRule{
				{ClaimType: "http://schemas.xmlsoap.org/claims/UPN", Source: spec.ClaimSourceUserAttribute, SourceKey: "preferred_username", Required: true},
				{ClaimType: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", Source: spec.ClaimSourceUserAttribute, SourceKey: "email"},
			},
		},
		CreatedAt: now,
	}
	return repo.Save(ctx, rp)
}
