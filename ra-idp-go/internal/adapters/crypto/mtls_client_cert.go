package crypto

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"strings"
)

type ParsedClientCertificate struct {
	SubjectDN      string
	ThumbprintS256 string
}

func ParseClientCertificateHeader(value string) (*ParsedClientCertificate, error) {
	decoded, err := url.QueryUnescape(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	var der []byte
	if block, _ := pem.Decode([]byte(decoded)); block != nil {
		der = block.Bytes
	} else {
		der, err = base64.StdEncoding.DecodeString(strings.Join(strings.Fields(decoded), ""))
		if err != nil {
			return nil, err
		}
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(cert.Raw)
	return &ParsedClientCertificate{
		SubjectDN:      cert.Subject.String(),
		ThumbprintS256: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

func ClientCertSubjectMatches(expected, presented string) bool {
	return normalizeDN(expected) == normalizeDN(presented)
}

func normalizeDN(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' })
	for i := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(parts[i]))
	}
	return strings.Join(parts, ",")
}
