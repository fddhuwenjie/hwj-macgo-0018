package domain

import (
	"strings"
)

func ValidateIDString(id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidCredentialID
	}
	if len(id) > 128 {
		return ErrInvalidCredentialID
	}
	return nil
}

func ValidateRequestKey(k RequestKey) error {
	if err := ValidateIDString(k.CallerID); err != nil {
		return ErrInvalidRequestKey
	}
	if err := ValidateIDString(k.NamespaceID); err != nil {
		return ErrInvalidRequestKey
	}
	if strings.TrimSpace(k.Key) == "" {
		return ErrInvalidRequestKey
	}
	if len(k.Key) > 256 {
		return ErrInvalidRequestKey
	}
	return nil
}

func ValidateDigestHash(hash string) error {
	if strings.TrimSpace(hash) == "" {
		return ErrInvalidDigest
	}
	if len(hash) > 256 {
		return ErrInvalidDigest
	}
	return nil
}

func ValidateAlgorithm(algorithm string) error {
	if strings.TrimSpace(algorithm) == "" {
		return ErrInvalidDigest
	}
	if len(algorithm) > 64 {
		return ErrInvalidDigest
	}
	return nil
}

func IsTerminalRequestState(s RequestState) bool {
	switch s {
	case RequestStateCommitted, RequestStateExpired:
		return true
	default:
		return false
	}
}

func IsTerminalCredentialState(s CredentialState) bool {
	switch s {
	case CredentialStateCommitted, CredentialStateExpired:
		return true
	default:
		return false
	}
}
