#!/usr/bin/env bash
# サーバーが localhost:8080 で起動済みの状態で実行してください。
# 使い方: ./demo.sh

set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
CLIENT_ID="${CLIENT_ID:-demo-client}"
CLIENT_SECRET="${DEMO_CLIENT_SECRET:-demo-client-secret}"
REDIRECT_URI="${REDIRECT_URI:-http://localhost:3000/callback}"
USERNAME="${DEMO_USERNAME:-alice}"
PASSWORD="${DEMO_USER_PASSWORD:-demo-password-1234}"
SCOPE="openid profile email offline_access"

TMP_DIR=$(mktemp -d)
COOKIE_JAR="$TMP_DIR/cookies.txt"
HEADERS="$TMP_DIR/headers.txt"
BODY="$TMP_DIR/body.txt"
trap 'rm -rf "$TMP_DIR"' EXIT

pp() {
  python3 -m json.tool 2>/dev/null || cat
}

json_get() {
  local field=$1
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

query_get() {
  local field=$1
  python3 -c \
    'import sys,urllib.parse; print(urllib.parse.parse_qs(urllib.parse.urlparse(sys.stdin.read().strip()).query).get(sys.argv[1], [""])[0])' \
    "$field"
}

header_value() {
  local name=$1
  awk -v name="$name" '
    BEGIN { IGNORECASE = 1 }
    $0 ~ "^" name ":" {
      sub("^[^:]+:[[:space:]]*", "")
      sub("\r$", "")
      value = $0
    }
    END { print value }
  ' "$HEADERS"
}

gen_pkce() {
  CODE_VERIFIER=$(openssl rand -hex 32)
  CODE_CHALLENGE=$(
    printf '%s' "$CODE_VERIFIER" |
      openssl dgst -sha256 -binary |
      openssl base64 -A |
      tr '+/' '-_' |
      tr -d '='
  )
}

token_request() {
  curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -X POST "$BASE/token" "$@"
}

echo "================================================================"
echo "  Regenerative Architecture Go - OAuth2 / OIDC IdP デモ"
echo "================================================================"

echo
echo "=== 1. Health ==="
curl -fsS "$BASE/health" | pp

echo
echo "=== 2. Discovery（OIDC メタデータ） ==="
curl -fsS "$BASE/.well-known/openid-configuration" | pp

echo
echo "=== 3. JWKS（公開鍵） ==="
curl -fsS "$BASE/jwks" | pp

echo
echo "=== 4. Authorization Code + PKCE ==="
gen_pkce
STATE="state-$(openssl rand -hex 8)"
NONCE="nonce-$(openssl rand -hex 8)"

curl -sS -D "$HEADERS" -o /dev/null -c "$COOKIE_JAR" -G "$BASE/authorize" \
  --data-urlencode "response_type=code" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "redirect_uri=$REDIRECT_URI" \
  --data-urlencode "scope=$SCOPE" \
  --data-urlencode "state=$STATE" \
  --data-urlencode "nonce=$NONCE" \
  --data-urlencode "code_challenge=$CODE_CHALLENGE" \
  --data-urlencode "code_challenge_method=S256"

LOCATION=$(header_value Location)
if [ "$LOCATION" != "/login" ]; then
  echo "ログイン画面へのリダイレクトがありません: $LOCATION" >&2
  exit 1
fi
echo "HttpOnly 認可トランザクションCookieを受領"

TRANSACTION=$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" "$BASE/api/auth/transaction")
CSRF_TOKEN=$(printf '%s' "$TRANSACTION" | json_get csrf_token)
echo "ログイン用CSRFトークンを受領"

LOGIN_RES=$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Origin: $BASE" \
  -X POST "$BASE/api/auth/login" \
  --data "$(printf '{"username":"%s","password":"%s"}' "$USERNAME" "$PASSWORD")")

NEXT=$(printf '%s' "$LOGIN_RES" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("next", ""))')
LOCATION=$(printf '%s' "$LOGIN_RES" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("redirect_to", ""))')
if [ "$NEXT" = "/consent" ]; then
  TRANSACTION=$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" "$BASE/api/auth/transaction")
  CSRF_TOKEN=$(printf '%s' "$TRANSACTION" | json_get csrf_token)
  echo "初回コンセントを許可"
  CONSENT_RES=$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $CSRF_TOKEN" \
    -H "Origin: $BASE" \
    -X POST "$BASE/api/auth/consent" \
    --data '{"action":"allow"}')
  LOCATION=$(printf '%s' "$CONSENT_RES" | json_get redirect_to)
fi

CODE=$(printf '%s' "$LOCATION" | query_get code)
RETURNED_STATE=$(printf '%s' "$LOCATION" | query_get state)
if [ -z "$CODE" ] || [ "$RETURNED_STATE" != "$STATE" ]; then
  echo "認可レスポンスが不正です: $LOCATION" >&2
  exit 1
fi
echo "認可コードを取得: ${CODE:0:30}..."

echo
echo "=== 5. 認可コードをトークンに交換 ==="
TOKEN_RES=$(token_request \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "code_verifier=$CODE_VERIFIER" \
  --data-urlencode "redirect_uri=$REDIRECT_URI")
printf '%s\n' "$TOKEN_RES" | pp
ACCESS_TOKEN=$(printf '%s' "$TOKEN_RES" | json_get access_token)
REFRESH_TOKEN=$(printf '%s' "$TOKEN_RES" | json_get refresh_token)

echo
echo "=== 6. UserInfo ==="
curl -fsS "$BASE/userinfo" -H "Authorization: Bearer $ACCESS_TOKEN" | pp

echo
echo "=== 7. Access Token Introspection ==="
curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/introspect" \
  --data-urlencode "token=$ACCESS_TOKEN" \
  --data-urlencode "token_type_hint=access_token" | pp

echo
echo "=== 8. Refresh Token Rotation ==="
REFRESH_RES=$(token_request \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH_TOKEN")
printf '%s\n' "$REFRESH_RES" | pp
NEW_REFRESH_TOKEN=$(printf '%s' "$REFRESH_RES" | json_get refresh_token)

echo
echo "=== 9. 旧 Refresh Token の再利用検出 ==="
curl -sS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/token" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH_TOKEN" | pp

echo
echo "=== 10. 認可コードの再利用検出 ==="
curl -sS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/token" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "code_verifier=$CODE_VERIFIER" \
  --data-urlencode "redirect_uri=$REDIRECT_URI" | pp

echo
echo "=== 11. 不正な Client Secret ==="
curl -sS -u "$CLIENT_ID:wrong-secret" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/token" \
  --data-urlencode "grant_type=client_credentials" | pp

echo
echo "=== 12. Client Credentials ==="
token_request \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "scope=openid" | pp

echo
echo "=== 13. PAR（Pushed Authorization Request） ==="
gen_pkce
PAR_RES=$(curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/par" \
  --data-urlencode "response_type=code" \
  --data-urlencode "redirect_uri=$REDIRECT_URI" \
  --data-urlencode "scope=openid" \
  --data-urlencode "code_challenge=$CODE_CHALLENGE" \
  --data-urlencode "code_challenge_method=S256")
printf '%s\n' "$PAR_RES" | pp
REQUEST_URI=$(printf '%s' "$PAR_RES" | json_get request_uri)

curl -sS -D "$HEADERS" -o "$BODY" -b "$COOKIE_JAR" -G "$BASE/authorize" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "request_uri=$REQUEST_URI"
PAR_LOCATION=$(header_value Location)
if [ -n "$PAR_LOCATION" ]; then
  echo "PAR 認可レスポンス: $PAR_LOCATION"
else
  echo "PAR 認可リクエストは追加の UI 操作を要求しました"
fi

echo
echo "=== 14. Refresh Token Revocation ==="
curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/revoke" \
  --data-urlencode "token=$NEW_REFRESH_TOKEN" \
  -o /dev/null
echo "revoke: HTTP 200"
curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/introspect" \
  --data-urlencode "token=$NEW_REFRESH_TOKEN" \
  --data-urlencode "token_type_hint=refresh_token" | pp

echo
echo "=== 15. Device Authorization Grant ==="
DEVICE_RES=$(curl -fsS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/device_authorization" \
  --data-urlencode "scope=openid profile")
printf '%s\n' "$DEVICE_RES" | pp
DEVICE_CODE=$(printf '%s' "$DEVICE_RES" | json_get device_code)
USER_CODE=$(printf '%s' "$DEVICE_RES" | json_get user_code)
DEVICE_INTERVAL=$(printf '%s' "$DEVICE_RES" | json_get interval)

echo "承認前のポーリング:"
curl -sS -u "$CLIENT_ID:$CLIENT_SECRET" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -X POST "$BASE/token" \
  --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
  --data-urlencode "device_code=$DEVICE_CODE" | pp

echo "ログイン済みセッションで user_code=$USER_CODE を承認..."
DEVICE_CONTEXT=$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  "$BASE/api/auth/device?user_code=$USER_CODE")
DEVICE_CSRF=$(printf '%s' "$DEVICE_CONTEXT" | json_get csrf_token)
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $DEVICE_CSRF" \
  -H "Origin: $BASE" \
  -X POST "$BASE/api/auth/device" \
  --data "$(printf '{"user_code":"%s","action":"allow"}' "$USER_CODE")" \
  -o /dev/null

echo "polling interval (${DEVICE_INTERVAL}s) 待機..."
sleep "$((DEVICE_INTERVAL + 1))"
token_request \
  --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
  --data-urlencode "device_code=$DEVICE_CODE" | pp

echo
echo "=== 16. 未登録 Redirect URI の拒否 ==="
curl -sS -G "$BASE/authorize" \
  --data-urlencode "response_type=code" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "redirect_uri=http://localhost:9999/callback" \
  --data-urlencode "scope=openid" \
  --data-urlencode "code_challenge=$CODE_CHALLENGE" \
  --data-urlencode "code_challenge_method=S256" | pp

echo
echo "================================================================"
echo "  デモ完了"
echo "================================================================"
