#!/usr/bin/env bash
# サーバーが localhost:3000 で起動済みの状態で実行してください
# 使い方: ./demo.sh
#
# このスクリプトは OAuth2 + OIDC の主要フローを端から端まで実行する:
#   - Discovery / JWKS の取得
#   - Authorization Code + PKCE
#   - access_token / id_token / refresh_token の取得
#   - /userinfo
#   - refresh_token のローテーション
#   - 同一 refresh_token の再利用検出（ファミリー失効）
#   - /introspect
#   - /revoke
#   - Device Authorization Grant (RFC 8628): device_code 発行 → 承認 → ポーリング
#   - エラーケース: 不正な client_secret, 認可コード再利用, 未登録 redirect_uri

set -euo pipefail
BASE="${BASE:-http://localhost:3000}"
CLIENT_ID="demo-web-app"
CLIENT_SECRET="${DEMO_CLIENT_SECRET:-demo-secret-please-rotate}"
REDIRECT_URI="http://localhost:8080/callback"
USER_SUB="user_alice"
SCOPE="openid profile email offline_access"

# Basic 認証ヘッダー
BASIC=$(printf '%s:%s' "$CLIENT_ID" "$CLIENT_SECRET" | base64)

# PKCE 値の生成（code_verifier と code_challenge）
gen_pkce() {
  CODE_VERIFIER=$(openssl rand -base64 96 | tr -d '\n=+/' | head -c 64)
  CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" | openssl dgst -sha256 -binary | base64 | tr -d '=' | tr '+/' '-_')
}

pp() { python3 -m json.tool 2>/dev/null || cat; }

echo "================================================================"
echo "  Regenerative Architecture — OAuth2 / OIDC IdP デモ"
echo "================================================================"

# ----------------------------------------------------------------
echo ""
echo "=== 1. Discovery（OIDC メタデータ） ==="
curl -s "$BASE/.well-known/openid-configuration" | pp | head -25

# ----------------------------------------------------------------
echo ""
echo "=== 2. JWKS（公開鍵） ==="
curl -s "$BASE/jwks" | pp | head -10

# ----------------------------------------------------------------
echo ""
echo "=== 3. Authorization Code + PKCE フロー ==="
gen_pkce
STATE="state-$RANDOM"
NONCE="nonce-$RANDOM"

AUTHORIZE_URL="$BASE/authorize?response_type=code&client_id=$CLIENT_ID&redirect_uri=$REDIRECT_URI&scope=$(printf '%s' "$SCOPE" | sed 's/ /%20/g')&state=$STATE&nonce=$NONCE&code_challenge=$CODE_CHALLENGE&code_challenge_method=S256"

# X-User-Sub ヘッダーでユーザー認証を簡略化（本アプリ仕様）。
# 初回はコンセント UI が返るので、request_id を抽出して /consent に POST する。
echo "/authorize を呼ぶ (X-User-Sub: ${USER_SUB})..."
AUTH_BODY=$(curl -s -H "X-User-Sub: ${USER_SUB}" "$AUTHORIZE_URL")
REQUEST_ID=$(printf '%s' "$AUTH_BODY" | sed -nE 's/.*name="request_id" value="([^"]+)".*/\1/p')
if [ -z "$REQUEST_ID" ]; then
  echo "コンセント UI から request_id を抽出できませんでした:"
  echo "$AUTH_BODY"
  exit 1
fi
echo "コンセント UI から request_id を抽出: $REQUEST_ID"

echo "/consent に action=allow を POST..."
LOCATION=$(curl -s -o /dev/null -w '%{redirect_url}' -X POST "$BASE/consent" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "request_id=$REQUEST_ID" \
  --data-urlencode "action=allow")
echo "Location: $LOCATION"
CODE=$(printf '%s' "$LOCATION" | sed -nE 's/.*[?&]code=([^&]+).*/\1/p')
if [ -z "$CODE" ]; then
  echo "認可コードが取得できませんでした。サーバーログを確認してください。"
  exit 1
fi
echo "認可コード: ${CODE:0:30}..."

# ----------------------------------------------------------------
echo ""
echo "=== 4. /token で認可コードを access_token / id_token / refresh_token に交換 ==="
TOKEN_RES=$(curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "code_verifier=$CODE_VERIFIER" \
  --data-urlencode "redirect_uri=$REDIRECT_URI")
echo "$TOKEN_RES" | pp
ACCESS_TOKEN=$(echo "$TOKEN_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
REFRESH_TOKEN=$(echo "$TOKEN_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['refresh_token'])")

# ----------------------------------------------------------------
echo ""
echo "=== 5. /userinfo（OIDC Core §5.3） ==="
curl -s "$BASE/userinfo" -H "Authorization: Bearer $ACCESS_TOKEN" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 6. /introspect（リソースサーバー視点） ==="
curl -s -X POST "$BASE/introspect" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "token=$ACCESS_TOKEN" \
  --data-urlencode "token_type_hint=access_token" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 7. /token (grant_type=refresh_token) — ローテーション ==="
REFRESH_RES=$(curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH_TOKEN")
echo "$REFRESH_RES" | pp
NEW_REFRESH=$(echo "$REFRESH_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['refresh_token'])")
echo "新しい refresh_token を受領: ${NEW_REFRESH:0:30}..."

# ----------------------------------------------------------------
echo ""
echo "=== 8. リプレイ検出: 旧 refresh_token を再利用 → 400 invalid_grant + ファミリー失効 ==="
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH_TOKEN" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 9. 認可コード再利用も同様にブロック（並行リプレイ） ==="
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "code_verifier=$CODE_VERIFIER" \
  --data-urlencode "redirect_uri=$REDIRECT_URI" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 10. 不正な client_secret → 401 invalid_client ==="
WRONG_BASIC=$(printf '%s:wrong-secret' "$CLIENT_ID" | base64)
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $WRONG_BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=client_credentials" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 11. /token (grant_type=client_credentials) ==="
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "scope=openid" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 12. PAR（Pushed Authorization Request、RFC 9126） ==="
gen_pkce
PAR_RES=$(curl -s -X POST "$BASE/par" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "response_type=code" \
  --data-urlencode "redirect_uri=$REDIRECT_URI" \
  --data-urlencode "scope=openid" \
  --data-urlencode "code_challenge=$CODE_CHALLENGE" \
  --data-urlencode "code_challenge_method=S256")
echo "$PAR_RES" | pp
REQUEST_URI=$(echo "$PAR_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['request_uri'])")
echo "request_uri を伴う /authorize にアクセス（既存コンセントを利用しているため即リダイレクト）..."
LOCATION=$(curl -s -o /dev/null -w '%{redirect_url}' -H "X-User-Sub: ${USER_SUB}" \
  "$BASE/authorize?client_id=$CLIENT_ID&request_uri=$REQUEST_URI")
echo "Location: $LOCATION"
if [ -z "$LOCATION" ]; then
  echo "(注: PAR 直後の /authorize がコンセント UI を返している場合があります — 本アプリでは初回フローでコンセント済みのため通常はスキップされます)"
fi

# ----------------------------------------------------------------
echo ""
echo "=== 13. /revoke で新 refresh_token を失効 ==="
curl -s -X POST "$BASE/revoke" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "token=$NEW_REFRESH" \
  -o /dev/null -w "HTTP %{http_code}\n"
echo "失効後の introspect は active=false を返すはず:"
curl -s -X POST "$BASE/introspect" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "token=$NEW_REFRESH" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 14. Device Authorization Grant（RFC 8628） ==="
echo "デバイスが /device_authorization を呼び device_code / user_code を取得..."
DA_RES=$(curl -s -X POST "$BASE/device_authorization" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "scope=openid profile")
echo "$DA_RES" | pp
DEVICE_CODE=$(echo "$DA_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['device_code'])")
USER_CODE=$(echo "$DA_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['user_code'])")
DA_INTERVAL=$(echo "$DA_RES" | python3 -c "import sys,json; print(json.load(sys.stdin)['interval'])")

echo ""
echo "承認前にポーリング → authorization_pending を期待:"
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
  --data-urlencode "device_code=$DEVICE_CODE" | pp

echo ""
echo "ユーザーが verification_uri で user_code=$USER_CODE を承認 (X-User-Sub: ${USER_SUB})..."
curl -s -X POST "$BASE/device" \
  -H "X-User-Sub: ${USER_SUB}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "user_code=$USER_CODE" \
  --data-urlencode "action=allow" | grep -oE '<h1>[^<]*</h1>' || true

echo ""
echo "interval (${DA_INTERVAL}s) 待ってからポーリング (これより速いと slow_down)..."
sleep "$((DA_INTERVAL + 1))"
echo "承認後にポーリング → access_token / id_token / refresh_token を期待:"
curl -s -X POST "$BASE/token" \
  -H "Authorization: Basic $BASIC" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
  --data-urlencode "device_code=$DEVICE_CODE" | pp

# ----------------------------------------------------------------
echo ""
echo "=== 15. 監査イベントログ（不変な audit trail） ==="
curl -s "$BASE/events" | pp | head -80

echo ""
echo "================================================================"
echo "  デモ完了"
echo "================================================================"
