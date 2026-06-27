// Package domain は ApplicationCatalog の aggregate 不変条件を所有する (wi-69)。
package domain

import (
	"errors"

	"ra-idp-go/internal/spec"
)

var (
	ErrNameRequired        = errors.New("application name is required")
	ErrInvalidKind         = errors.New("invalid application kind")
	ErrInvalidStatus       = errors.New("invalid application status")
	ErrWeblinkLaunchURL    = errors.New("weblink application requires launch_url")
	ErrWeblinkNoBindings   = errors.New("weblink application must not have protocol bindings")
	ErrInvalidBindingType  = errors.New("invalid protocol binding type")
	ErrOIDCBindingClientID = errors.New("oidc binding requires client_id")
	ErrWsFedBindingWtrealm = errors.New("wsfed binding requires wtrealm")
)

// ValidateApplication は Application aggregate の不変条件を検証する (wi-69)。
// weblink は launch_url 必須で binding を持てない。federated は binding を検証する。
func ValidateApplication(app *spec.Application) error {
	if app.Name == "" {
		return ErrNameRequired
	}
	if !app.Kind.Valid() {
		return ErrInvalidKind
	}
	if !app.Status.Valid() {
		return ErrInvalidStatus
	}
	if app.Kind == spec.ApplicationWeblink {
		if app.LaunchURL == "" {
			return ErrWeblinkLaunchURL
		}
		if len(app.Bindings) > 0 {
			return ErrWeblinkNoBindings
		}
	}
	for _, binding := range app.Bindings {
		if err := ValidateBinding(binding); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBinding は 1 つの protocol binding を検証する (wi-69)。
// oidc は client_id、wsfed は wtrealm を必須とする。
func ValidateBinding(binding spec.ProtocolBinding) error {
	if !binding.Type.Valid() {
		return ErrInvalidBindingType
	}
	switch binding.Type {
	case spec.ProtocolBindingOIDC:
		if binding.ClientID == "" {
			return ErrOIDCBindingClientID
		}
	case spec.ProtocolBindingWsFed:
		if binding.Wtrealm == "" {
			return ErrWsFedBindingWtrealm
		}
	}
	return nil
}
