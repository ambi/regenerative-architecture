package bootstrap

import (
	"errors"
	"os"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/adapters/policy"
)

func assembleAuthorizer() (oauthports.Authorizer, error) {
	switch envDefault("AUTHZEN", "local") {
	case "local":
		return policy.Local{}, nil
	case "remote":
		endpoint := os.Getenv("AUTHZEN_URL")
		if endpoint == "" {
			return nil, errors.New("AUTHZEN=remote requires AUTHZEN_URL")
		}
		return policy.NewRemote(endpoint), nil
	default:
		return nil, errors.New("AUTHZEN must be local or remote")
	}
}
