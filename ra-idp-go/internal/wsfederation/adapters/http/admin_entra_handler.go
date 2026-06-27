package http

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"
	feddomain "ra-idp-go/internal/wsfederation/domain"

	"github.com/labstack/echo/v5"
)

const defaultEntraReplyURL = "https://login.microsoftonline.com/login.srf"

type configureEntraRequest struct {
	Domain                string `json:"domain"`
	IssuerURI             string `json:"issuer_uri"`
	SourceAnchorAttribute string `json:"source_anchor_attribute"`
	ReplyURL              string `json:"reply_url"`
}

func (r configureEntraRequest) validate() error {
	switch {
	case strings.TrimSpace(r.Domain) == "":
		return errBadRequest("domain is required")
	case strings.Contains(strings.TrimSpace(r.Domain), "://"):
		return errBadRequest("domain must be a verified DNS domain, not a URL")
	}
	if strings.TrimSpace(r.SourceAnchorAttribute) == "" {
		return errBadRequest("source_anchor_attribute is required")
	}
	if strings.TrimSpace(r.ReplyURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(r.ReplyURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return errBadRequest("reply_url must be an absolute URL")
		}
	}
	return nil
}

func (d Deps) handleConfigureEntraFederation(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req configureEntraRequest
	if err := c.Bind(&req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSON が不正です")
	}
	if err := req.validate(); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}

	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)
	sourceAttr := strings.TrimSpace(req.SourceAnchorAttribute)
	if err := d.validateEntraSourceAnchors(c, sourceAttr); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}

	base := strings.TrimRight(core.RequestIssuer(c, d.Issuer), "/")
	passive := base + core.TenantRoute(c, "/wsfed")
	active := base + core.TenantRoute(c, "/trust/usernamemixed")
	mex := base + core.TenantRoute(c, "/trust/mex")
	metadata := base + core.TenantRoute(c, "/federationmetadata/2007-06/federationmetadata.xml")
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	issuerURI := strings.TrimSpace(req.IssuerURI)
	if issuerURI == "" {
		issuerURI = "urn:ra-idp:entra:" + domain
	}
	replyURL := strings.TrimSpace(req.ReplyURL)
	if replyURL == "" {
		replyURL = defaultEntraReplyURL
	}

	profile := &spec.EntraFederationProfile{
		Domain:                domain,
		IssuerURI:             issuerURI,
		SourceAnchorAttribute: sourceAttr,
		ImmutableIDAttribute:  feddomain.EntraImmutableIDAttribute,
		PassiveLogOnURI:       passive,
		ActiveLogOnURI:        active,
		MetadataExchangeURI:   mex,
	}
	now := time.Now().UTC()
	existing, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, issuerURI)
	if err != nil {
		return err
	}
	rp := &spec.WsFedRelyingParty{
		TenantID:     tenantID,
		Wtrealm:      issuerURI,
		DisplayName:  "Microsoft Entra federation: " + domain,
		ReplyURLs:    []string{replyURL},
		Audience:     issuerURI,
		TokenType:    spec.TokenTypeSAML11,
		ClaimPolicy:  feddomain.BuildEntraClaimPolicy(),
		EntraProfile: profile,
		CreatedAt:    now,
	}
	status := http.StatusCreated
	if existing != nil {
		rp.CreatedAt = existing.CreatedAt
		rp.UpdatedAt = &now
		status = http.StatusOK
	}
	if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
		return err
	}
	d.emit(&spec.EntraFederationConfigured{At: now, TenantID: tenantID, Domain: domain, IssuerURI: issuerURI})

	return core.NoStoreJSON(c, status, map[string]any{
		"profile":       rp.EntraProfile,
		"relying_party": rp,
		"powershell": map[string]string{
			"IssuerUri":           issuerURI,
			"PassiveLogOnUri":     passive,
			"ActiveLogOnUri":      active,
			"MetadataExchangeUri": mex,
			"SigningCertificate":  "Use the X.509 certificate published in " + metadata,
		},
		"known_limitations": []string{
			"Hybrid Azure AD Join device registration is not provided; use managed/PHS or AD FS coexistence when device registration is required.",
		},
	})
}

func (d Deps) validateEntraSourceAnchors(c *echo.Context, sourceAttr string) error {
	users, err := d.UserRepo.FindAll(c.Request().Context(), core.RequestTenantID(c))
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for _, user := range users {
		attrs := feddomain.ResolveUserAttributes(*user)
		withProfile, err := feddomain.ApplyEntraProfile(attrs, &spec.EntraFederationProfile{SourceAnchorAttribute: sourceAttr})
		if err != nil {
			return errBadRequest("sourceAnchor validation failed for user " + user.Sub + ": " + err.Error())
		}
		immutableID := withProfile[feddomain.EntraImmutableIDAttribute][0]
		if previous, ok := seen[immutableID]; ok {
			return errBadRequest("sourceAnchor attribute " + sourceAttr + " is not unique: users " + previous + " and " + user.Sub)
		}
		seen[immutableID] = user.Sub
	}
	return nil
}
