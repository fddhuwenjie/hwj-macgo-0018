package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// AppExecErrorKind describes the coarse category of an application-level failure.
type AppExecErrorKind string

const (
	AppExecErrorKindInvalidInput AppExecErrorKind = "invalid_input"
	AppExecErrorKindConflict     AppExecErrorKind = "conflict"
	AppExecErrorKindNotFound     AppExecErrorKind = "not_found"
	AppExecErrorKindStaleVersion AppExecErrorKind = "stale_version"
	AppExecErrorKindPermission   AppExecErrorKind = "permission"
	AppExecErrorKindCancelled    AppExecErrorKind = "cancelled"
	AppExecErrorKindUnavailable  AppExecErrorKind = "unavailable"
)

// AppExecError is an application-layer error that keeps the original cause,
// the operation that produced it, and a coarse classification for callers.
type AppExecError struct {
	Kind AppExecErrorKind
	Op   string
	Err  error
}

func (e *AppExecError) Error() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	b.WriteString("application[")
	b.WriteString(string(e.Kind))
	b.WriteString("]")
	if e.Op != "" {
		b.WriteString(" op=")
		b.WriteString(e.Op)
	}
	if e.Err != nil {
		b.WriteString(" cause=")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *AppExecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewAppExecError classifies err. If err is nil the returned error still
// carries the kind, which is useful when callers need a stable category.
func NewAppExecError(kind AppExecErrorKind, op string, err error) error {
	if err == nil {
		return &AppExecError{Kind: kind, Op: op}
	}
	return &AppExecError{Kind: kind, Op: op, Err: err}
}

// IsAppExecError reports whether err is an AppExecError of the given kind.
func IsAppExecError(err error, kind AppExecErrorKind) bool {
	var target *AppExecError
	if !errors.As(err, &target) {
		return false
	}
	return target != nil && target.Kind == kind
}

// AsAppExecError returns the underlying AppExecError if err wraps one.
func AsAppExecError(err error) (*AppExecError, bool) {
	var target *AppExecError
	if errors.As(err, &target) {
		return target, target != nil
	}
	return nil, false
}

// NormalizeAppExecError converts a plain error into an AppExecError when it
// is not already an AppExecError. It is useful at application service bounds.
func NormalizeAppExecError(kind AppExecErrorKind, op string, err error) error {
	if err == nil {
		return nil
	}
	var target *AppExecError
	if errors.As(err, &target) {
		return target
	}
	return NewAppExecError(kind, op, err)
}

// IsExternalCancellation reports whether the error chain contains
// context.Canceled.
func IsExternalCancellation(err error) bool {
	return errors.Is(err, context.Canceled)
}

// FormatDebug provides a compact debugging representation without leaking
// transport-specific details.
func FormatAppExecError(err error) string {
	if err == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", err)
}
