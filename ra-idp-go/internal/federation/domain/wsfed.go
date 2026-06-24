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
	"strings"

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
