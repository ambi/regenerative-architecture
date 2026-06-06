package usecases

import "fmt"

// OAuthError は redirect 経由で返すべき OAuth2 規定のエラー。
// HTTP 層が code/description を redirect_uri クエリに展開する。
type OAuthError struct {
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Description)
}

func NewOAuthError(code, description string) *OAuthError {
	return &OAuthError{Code: code, Description: description}
}
