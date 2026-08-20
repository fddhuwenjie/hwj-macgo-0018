package application

import (
	"context"
	"errors"
	"fmt"
)

// ScopeTransaction is a minimal transaction handle returned by a
// ScopeTransactionManager. Application services use this contract instead of
// depending on a concrete repository transaction type, which keeps the
// transaction boundary explicit and testable.
type ScopeTransaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ScopeTransactionManager begins transaction-scoped work. Application
// transactions are expected to be short-lived and cancellation-aware.
type ScopeTransactionManager interface {
	Begin(ctx context.Context) (context.Context, ScopeTransaction, error)
}

// ScopeTransactionFunc is the callback executed inside one transaction.
type ScopeTransactionFunc func(ctx context.Context) error

// ScopeRunner executes application code inside a transaction and converts
// rollback/commit errors into stable AppExecError kinds.
type ScopeRunner struct {
	txManager ScopeTransactionManager
}

// NewScopeRunner creates a transaction-scoped runner.
func NewScopeRunner(manager ScopeTransactionManager) *ScopeRunner {
	return &ScopeRunner{txManager: manager}
}

// Run executes fn inside a transaction. A non-nil error from fn triggers a
// rollback. A commit failure also attempts a defensive rollback.
func (r *ScopeRunner) Run(ctx context.Context, fn ScopeTransactionFunc) (err error) {
	if r == nil || r.txManager == nil {
		return NewAppExecError(AppExecErrorKindUnavailable, "scope_transaction.Run", errors.New("transaction manager is nil"))
	}
	if fn == nil {
		return NewAppExecError(AppExecErrorKindInvalidInput, "scope_transaction.Run", errors.New("transaction function is nil"))
	}

	ctx, tx, err := r.txManager.Begin(ctx)
	if err != nil {
		return NewAppExecError(AppExecErrorKindUnavailable, "scope_transaction.Begin", err)
	}

	defer func() {
		if rec := recover(); rec != nil {
			_ = tx.Rollback(ctx)
			panic(rec)
		}
	}()

	if fnErr := fn(ctx); fnErr != nil {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil {
			// A rollback failure is secondary to the original cause; keep the
			// primary error's identity intact so callers can still match it
			// with errors.Is/As (e.g. context.Canceled).
			return NewAppExecError(AppExecErrorKindUnavailable, "scope_transaction.Rollback", combineErrors(fnErr, rollbackErr))
		}
		// Wrap fnErr directly instead of flattening it to a string, so the
		// cancellation identity (context.Canceled / context.DeadlineExceeded)
		// survives the transaction boundary and a caller can errors.Is it.
		return NewAppExecError(classifyRunError(fnErr), "scope_transaction.Run", fnErr)
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil {
			return NewAppExecError(AppExecErrorKindUnavailable, "scope_transaction.CommitRollback", combineErrors(commitErr, rollbackErr))
		}
		return NewAppExecError(AppExecErrorKindUnavailable, "scope_transaction.Commit", commitErr)
	}
	return nil
}

// RunInTransaction is a convenience alias for callers that read naturally.
func (r *ScopeRunner) RunInTransaction(ctx context.Context, fn ScopeTransactionFunc) error {
	return r.Run(ctx, fn)
}

// NoopScopeTransaction is useful for self-check and wiring tests.
type NoopScopeTransaction struct{}

func (NoopScopeTransaction) Commit(_ context.Context) error   { return nil }
func (NoopScopeTransaction) Rollback(_ context.Context) error { return nil }

// StaticScopeTransactionManager always returns a noop transaction. It is not
// meant for production persistence; it keeps application wiring simple.
type StaticScopeTransactionManager struct{}

func (StaticScopeTransactionManager) Begin(ctx context.Context) (context.Context, ScopeTransaction, error) {
	return ctx, NoopScopeTransaction{}, nil
}

var _ ScopeTransactionManager = StaticScopeTransactionManager{}

func combineErrors(primary, secondary error) error {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	// Wrap the primary cause with %w so its identity is preserved even when a
	// rollback failure is reported alongside it.
	return fmt.Errorf("%w; rollback: %v", primary, secondary)
}

func classifyRunError(err error) AppExecErrorKind {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return AppExecErrorKindCancelled
	}
	return AppExecErrorKindUnavailable
}
