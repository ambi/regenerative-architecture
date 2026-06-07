package usecases

import (
	"slices"
	"strings"
)

const (
	ACRPassword = "urn:ra-idp:acr:pwd"
	ACRMFA      = "urn:ra-idp:acr:mfa"
)

var mfaAMRValues = []string{"otp", "webauthn", "hwk", "swk"}

func DeriveACR(amr []string) string {
	for _, method := range amr {
		if slices.Contains(mfaAMRValues, method) {
			return ACRMFA
		}
	}
	return ACRPassword
}

func ACRSatisfies(current, requested string) bool {
	for _, value := range strings.Fields(requested) {
		if value == current || current == ACRMFA && value == ACRPassword {
			return true
		}
	}
	return false
}
