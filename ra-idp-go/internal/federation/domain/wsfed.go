// WS-Federation passive requestor profile のリクエスト解析と検証 (wi-61)。
//
// 本ファイルは HTTP やトークン署名に依存しない純粋なドメイン判定を担う:
//
//   - ParseSignInRequest: passive のクエリ (wa / wtrealm / wreply / wctx / wfresh / wauth / whr) を解析する。
//   - ValidateSignIn: 要求を登録済み RP に解決し、wreply を許可集合に限定する (open redirect 防止, fail-closed)。
//
// 判定不能・不一致はすべて拒否側へ倒す。
package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"ra-idp-go/internal/spec"
)

// WS-Federation の wa アクション値。
const (
	WaSignIn         = "wsignin1.0"
	WaSignOut        = "wsignout1.0"
	WaSignOutCleanup = "wsignoutcleanup1.0"
)

// WsFedSignInRequest は passive sign-in のクエリパラメータ。
type WsFedSignInRequest struct {
	Wa      string // アクション (wsignin1.0)。
	Wtrealm string // RP 識別子。
	Wreply  string // 任意。返信先 URL。
	Wctx    string // 任意。RP が往復させる不透明コンテキスト。
	Wfresh  string // 任意。再認証の最大経過時間 (分)。
	Wauth   string // 任意。要求された認証方式 URI。
	Whr     string // 任意。home realm のヒント。
}

// ParseSignInRequest はパラメータ参照関数から sign-in 要求を構築する。
// get は欠落時に空文字列を返すこと (url.Values.Get と同じ契約)。
func ParseSignInRequest(get func(string) string) WsFedSignInRequest {
	return WsFedSignInRequest{
		Wa:      strings.TrimSpace(get("wa")),
		Wtrealm: strings.TrimSpace(get("wtrealm")),
		Wreply:  strings.TrimSpace(get("wreply")),
		Wctx:    get("wctx"),
		Wfresh:  strings.TrimSpace(get("wfresh")),
		Wauth:   strings.TrimSpace(get("wauth")),
		Whr:     strings.TrimSpace(get("whr")),
	}
}

// IsSignIn は要求が sign-in アクションかを返す。
func (r WsFedSignInRequest) IsSignIn() bool { return r.Wa == WaSignIn }

// ValidatedSignIn は検証を通った sign-in 要求の確定結果。
type ValidatedSignIn struct {
	RelyingParty spec.WsFedRelyingParty
	ReplyURL     string // 実際に POST する返信先 (許可集合内に確定済み)。
	Wctx         string // 往復させる不透明コンテキスト。
}

// ValidateSignIn は要求を RP に解決し、wreply を許可集合に限定する (fail-closed)。
//
//   - wa は wsignin1.0 でなければならない。
//   - wtrealm は rp.Wtrealm と完全一致しなければならない。
//   - wreply 指定時は rp.ReplyURLs の完全一致のみ受理する (open redirect 防止)。
//   - wreply 省略時は rp.ReplyURLs の先頭を既定の返信先とする。
func ValidateSignIn(req WsFedSignInRequest, rp spec.WsFedRelyingParty) (ValidatedSignIn, error) {
	if req.Wa != WaSignIn {
		return ValidatedSignIn{}, fmt.Errorf("wsfed: unsupported wa %q", req.Wa)
	}
	if req.Wtrealm == "" {
		return ValidatedSignIn{}, fmt.Errorf("wsfed: wtrealm is required")
	}
	if req.Wtrealm != rp.Wtrealm {
		return ValidatedSignIn{}, fmt.Errorf("wsfed: wtrealm %q does not match relying party", req.Wtrealm)
	}
	if len(rp.ReplyURLs) == 0 {
		return ValidatedSignIn{}, fmt.Errorf("wsfed: relying party %q has no reply URLs", rp.Wtrealm)
	}

	replyURL, err := resolveReplyURL(req.Wreply, rp.ReplyURLs)
	if err != nil {
		return ValidatedSignIn{}, err
	}

	return ValidatedSignIn{RelyingParty: rp, ReplyURL: replyURL, Wctx: req.Wctx}, nil
}

// freshAuthGrace は wfresh 判定でログイン往復を吸収する猶予 (wi-61)。
//
// wfresh=0 (強制再認証) で厳密に now-authTime>0 を要求すると、再ログイン直後の戻り
// (authTime≈now、ただし秒単位で数秒のずれ) でも再認証ループに陥る。直近の再ログインを
// 満たすために短い猶予を設ける。猶予より古いセッションは wfresh=0 で再認証を強制する。
const freshAuthGrace = 30 * time.Second

// RequiresFreshAuth は wfresh (分) に対して authTime が古すぎ、再認証が必要かを返す。
//
//   - wfresh 未指定・負数・解析不能なら制約なし (false)。
//   - wfresh=N (分) は now-authTime > N 分で再認証を要求する。
//   - wfresh=0 は強制再認証だが、ログイン往復を吸収する猶予 (freshAuthGrace) 内のセッションは許容する。
func RequiresFreshAuth(wfresh string, authTime, now time.Time) bool {
	minutes, ok := parseWfresh(wfresh)
	if !ok {
		return false
	}
	maxAge := max(time.Duration(minutes)*time.Minute, freshAuthGrace)
	return now.Sub(authTime) > maxAge
}

func parseWfresh(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// AuthnMethodClass はプロトコル非依存の認証方式クラス (wi-61)。SAML 1.1/2.0 の方式 URI へは
// assertion 構築時に変換する。
type AuthnMethodClass string

const (
	// AuthnPassword はパスワード (フォーム) 認証。
	AuthnPassword AuthnMethodClass = "password"
	// AuthnUnspecified は方式を特定しない。
	AuthnUnspecified AuthnMethodClass = "unspecified"
)

// この IdP が満たせる wauth (要求された認証方式 URI) と、満たせない (= 別 WI 範囲) URI。
// 満たせない方式が要求された場合は fail-closed で拒否する。
var (
	passwordWauthURIs = map[string]struct{}{
		"urn:oasis:names:tc:SAML:1.0:am:password":                                        {},
		"urn:oasis:names:tc:SAML:2.0:ac:classes:Password":                                {},
		"urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport":              {},
		"http://schemas.microsoft.com/ws/2008/06/identity/authenticationmethod/password": {},
	}
	// 統合 Windows 認証 (WIA/Negotiate/Kerberos) はフォーム IdP では満たせない。
	// 無音サインインは [[wi-65-kerberos-spnego-inbound-silent-sso]] が担う。
	integratedWauthURIs = map[string]struct{}{
		"urn:federation:authentication:windows":                                         {},
		"urn:oasis:names:tc:SAML:2.0:ac:classes:Kerberos":                               {},
		"http://schemas.microsoft.com/ws/2008/06/identity/authenticationmethod/windows": {},
	}
)

// ResolveAuthnMethod は実施した認証 (amr) と要求された wauth から、発行 assertion に載せる
// 認証方式クラスを決める (wi-61)。
//
//   - wauth 未指定なら実施した方式をそのまま用いる。
//   - wauth が password 系で、実施が password なら満たし、password を用いる。
//   - wauth が統合 Windows 認証など本 IdP で満たせない方式なら error を返す (fail-closed)。
//   - wauth が未知の URI なら hint とみなし、実施した方式を用いる。
func ResolveAuthnMethod(wauth string, amr []string) (AuthnMethodClass, error) {
	performed := classFromAMR(amr)
	wauth = strings.TrimSpace(wauth)
	if wauth == "" {
		return performed, nil
	}
	if _, ok := integratedWauthURIs[wauth]; ok {
		return "", fmt.Errorf("wsfed: requested authentication method %q is not supported by this identity provider", wauth)
	}
	if _, ok := passwordWauthURIs[wauth]; ok {
		if performed != AuthnPassword {
			return "", fmt.Errorf("wsfed: requested authentication method %q was not satisfied", wauth)
		}
		return AuthnPassword, nil
	}
	return performed, nil
}

func classFromAMR(amr []string) AuthnMethodClass {
	if slices.Contains(amr, "pwd") {
		return AuthnPassword
	}
	return AuthnUnspecified
}

// resolveReplyURL は wreply を許可集合に対して fail-closed に解決する。
func resolveReplyURL(wreply string, allowed []string) (string, error) {
	if wreply == "" {
		return allowed[0], nil
	}
	if slices.Contains(allowed, wreply) {
		return wreply, nil
	}
	return "", fmt.Errorf("wsfed: wreply %q is not an allowed reply URL", wreply)
}
