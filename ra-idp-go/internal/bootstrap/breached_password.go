package bootstrap

import (
	"fmt"
	"log"
	"strings"

	authnports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/infrastructure/policy"
)

// breachedPasswordCheckerVersion は HIBP の User-Agent に乗せる版番号 (HIBP の etiquette)。
const breachedPasswordCheckerVersion = "0.3.0"

// resolveBreachedPasswordChecker は BREACHED_PASSWORD_CHECKER 環境変数から
// BreachedPasswordChecker adapter を組み立てる。既定は noop (外部依存なし)。
// hibp 選択時は api.pwnedpasswords.com への egress が要る (ADR-028 §3)。
func resolveBreachedPasswordChecker(getenv func(string) string) (authnports.BreachedPasswordChecker, error) {
	kind := strings.ToLower(strings.TrimSpace(getenv("BREACHED_PASSWORD_CHECKER")))
	if kind == "" {
		kind = "noop"
	}
	switch kind {
	case "noop":
		log.Printf("breached password checker: noop")
		return policy.NoopBreachedPasswordChecker{}, nil
	case "hibp":
		log.Printf("breached password checker: hibp")
		return policy.NewHibpBreachedPasswordChecker(breachedPasswordCheckerVersion), nil
	default:
		return nil, fmt.Errorf("unsupported BREACHED_PASSWORD_CHECKER=%q (want noop or hibp)", kind)
	}
}
