package application

import (
	"fmt"
	"time"

	"hwj-macgo-0018/domain"
)

// lateCommitError is the single classification point for a credential from an
// obsolete execution generation. Keeping it here makes the commit boundary
// explicit instead of leaking a generic conflict to callers.
func lateCommitError(supplied, current int64) error {
	if supplied != current {
		return fmt.Errorf("%w: supplied generation %d, current generation %d", domain.ErrLateCommit, supplied, current)
	}
	return nil
}

func activeCredentialError(status string, expiresAt, now time.Time) error {
	if status != credentialActive {
		return ErrIllegalTransition
	}
	if now.After(expiresAt) {
		return ErrExpired
	}
	return nil
}
