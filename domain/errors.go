package domain

import "errors"

var (
	ErrInvalidTransition   = errors.New("domain: invalid state transition")
	ErrConflictRequest     = errors.New("domain: conflicting request")
	ErrDuplicateDigest     = errors.New("domain: duplicate operation digest for request key")
	ErrExpiredLease        = errors.New("domain: lease expired")
	ErrStaleVersion        = errors.New("domain: stale version")
	ErrNoCredential        = errors.New("domain: no execution credential")
	ErrInvalidRequestKey   = errors.New("domain: invalid request key")
	ErrInvalidCredentialID = errors.New("domain: invalid credential id")
	ErrInvalidDigest       = errors.New("domain: invalid operation digest")
	ErrAlreadyCommitted    = errors.New("domain: result already committed")
	ErrTakeoverNotAllowed  = errors.New("domain: takeover not allowed")
	ErrLateCommit          = errors.New("domain: late commit from old generation")
	ErrInvalidLease        = errors.New("domain: invalid lease")
	ErrInvalidState        = errors.New("domain: invalid state")
	ErrInvalidCaller       = errors.New("domain: invalid caller")
	ErrInvalidNamespace    = errors.New("domain: invalid namespace")
)
