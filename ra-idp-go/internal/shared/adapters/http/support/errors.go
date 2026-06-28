package support

// OAuthErrorBody は OAuth 2.0 のエラーレスポンス body (RFC 6749 §5.2) を組み立てる。
// テナント解決 middleware や各コンテキストのエラー出力が共有する。
func OAuthErrorBody(code, description string) map[string]string {
	return map[string]string{"error": code, "error_description": description}
}
