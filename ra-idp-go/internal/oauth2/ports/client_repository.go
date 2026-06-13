// Package ports: OAuth2 ユースケースが要求する境界。
package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type ClientRepository interface {
	FindByID(ctx context.Context, clientID string) (*spec.Client, error)
	Save(ctx context.Context, c *spec.Client) error
	Delete(ctx context.Context, clientID string) error
	FindAll(ctx context.Context) ([]*spec.Client, error)
}

type UserRepository interface {
	FindBySub(ctx context.Context, sub string) (*spec.User, error)
	FindByUsername(ctx context.Context, username string) (*spec.User, error)
	FindByEmail(ctx context.Context, email string) (*spec.User, error)
	FindAll(ctx context.Context) ([]*spec.User, error)
	Save(ctx context.Context, user *spec.User) error
}

type ConsentRepository interface {
	Find(ctx context.Context, sub, clientID string) (*spec.Consent, error)
	Save(ctx context.Context, c *spec.Consent) error
	Revoke(ctx context.Context, sub, clientID string) error
}
