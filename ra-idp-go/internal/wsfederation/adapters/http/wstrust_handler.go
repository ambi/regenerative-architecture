package http

import (
	"io"
	"net/http"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"
	"ra-idp-go/internal/wsfederation/adapters/wstrust"
	feddomain "ra-idp-go/internal/wsfederation/domain"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleWsTrustUsernameMixed(c *echo.Context) error {
	now := time.Now().UTC()
	tenantID := core.RequestTenantID(c)
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return err
	}
	rst, err := wstrust.ParseRST(body, now)
	if err != nil {
		d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, Reason: err.Error()})
		return c.String(http.StatusBadRequest, err.Error())
	}
	if ok, err := d.recordWsTrustMessageID(c, rst.MessageID, now); err != nil {
		return err
	} else if !ok {
		d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Reason: "replayed MessageID"})
		return c.String(http.StatusBadRequest, "replayed MessageID")
	}

	rp, err := d.WsFedRPRepo.FindByWtrealm(c.Request().Context(), tenantID, rst.AppliesTo)
	if err != nil {
		return err
	}
	if rp == nil {
		d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Reason: "unknown relying party"})
		return c.String(http.StatusBadRequest, "unknown relying party")
	}
	user, err := d.authenticateWsTrustUser(c, rst.Username, rst.Password, now)
	if err != nil {
		d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Reason: err.Error()})
		return c.String(http.StatusUnauthorized, "invalid credentials")
	}

	result, err := feddomain.IssueClaims(rp.ClaimPolicy, feddomain.ResolveUserAttributes(*user))
	if err != nil {
		d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Reason: "claim issuance failed"})
		return c.String(http.StatusInternalServerError, "claim issuance failed")
	}
	tokenType := rp.EffectiveTokenType()
	if strings.TrimSpace(rst.TokenType) != "" {
		if rst.TokenType != string(spec.TokenTypeSAML11) && rst.TokenType != string(spec.TokenTypeSAML20) {
			d.emit(&spec.WsTrustTokenRejected{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Reason: "unsupported token type"})
			return c.String(http.StatusBadRequest, "unsupported token type")
		}
		tokenType = spec.WsFedTokenType(rst.TokenType)
	}
	signed, _, err := samltoken.BuildSignedAssertion(samltoken.AssertionInput{
		Version:      samlVersion(tokenType),
		Issuer:       core.RequestIssuer(c, d.Issuer),
		Audience:     rp.EffectiveAudience(),
		Recipient:    rst.AppliesTo,
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(assertionLifetime),
		AuthnInstant: now,
		AuthnMethod:  feddomain.AuthnPassword,
		Result:       result,
	}, d.FederationSigner)
	if err != nil {
		return c.String(http.StatusInternalServerError, "assertion build failed")
	}
	out, err := wstrust.BuildRSTR(signed, rst.MessageID, rst.AppliesTo, string(tokenType), now, now.Add(assertionLifetime))
	if err != nil {
		return c.String(http.StatusInternalServerError, "rstr build failed")
	}
	d.emit(&spec.WsTrustTokenIssued{At: now, TenantID: tenantID, AppliesTo: rst.AppliesTo, Sub: user.Sub})
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "application/soap+xml; charset=utf-8", out)
}

func (d Deps) recordWsTrustMessageID(c *echo.Context, messageID string, now time.Time) (bool, error) {
	if d.ClientAssertionReplayStore == nil {
		return true, nil
	}
	return d.ClientAssertionReplayStore.RecordIfNew(c.Request().Context(), "wstrust:"+messageID, int(assertionLifetime.Seconds()), now)
}

func (d Deps) authenticateWsTrustUser(c *echo.Context, username, password string, now time.Time) (*spec.User, error) {
	normalizedUsername := strings.ToLower(username)
	if d.LoginAttemptThrottle != nil {
		result, err := d.LoginAttemptThrottle.TryAcquire(c.Request().Context(), authports.LoginThrottleAccount, normalizedUsername, now)
		if err != nil {
			return nil, err
		}
		if !result.Allowed {
			return nil, errBadRequest("login throttled")
		}
	}
	user, err := d.UserRepo.FindByUsername(c.Request().Context(), core.RequestTenantID(c), username)
	if err != nil {
		return nil, err
	}
	hashToVerify := d.SentinelPasswordHash
	if user != nil {
		hashToVerify = user.PasswordHash
	}
	ok := false
	if hashToVerify != "" && d.PasswordHasher != nil {
		ok, err = d.PasswordHasher.Verify(password, hashToVerify)
	}
	if user == nil || err != nil || !ok || !user.IsActive() {
		if d.LoginAttemptThrottle != nil {
			_, _ = d.LoginAttemptThrottle.RecordFailure(c.Request().Context(), authports.LoginThrottleAccount, normalizedUsername, now)
		}
		return nil, errBadRequest("invalid credentials")
	}
	if d.LoginAttemptThrottle != nil {
		if err := d.LoginAttemptThrottle.RecordSuccess(c.Request().Context(), authports.LoginThrottleAccount, normalizedUsername); err != nil {
			return nil, err
		}
	}
	return user, nil
}
