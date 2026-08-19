package domain

import "time"

type CredentialState string

const (
	CredentialStateIssued    CredentialState = "issued"
	CredentialStateOccupied  CredentialState = "occupied"
	CredentialStateCommitted CredentialState = "committed"
	CredentialStateFailed    CredentialState = "failed"
	CredentialStateTakenOver CredentialState = "taken_over"
	CredentialStateExpired   CredentialState = "expired"
)

type ExecutionCredential struct {
	ID                string
	RequestKey        RequestKey
	OperationDigestID string
	LeaseID           string
	Generation        int
	State             CredentialState
	ResultSnapshotID  string
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewExecutionCredential(id string, key RequestKey, digestID string, generation int, now time.Time) (*ExecutionCredential, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateIDString(digestID); err != nil {
		return nil, ErrInvalidDigest
	}
	if generation < 1 {
		return nil, ErrInvalidState
	}
	return &ExecutionCredential{
		ID:                id,
		RequestKey:        key,
		OperationDigestID: digestID,
		Generation:        generation,
		State:             CredentialStateIssued,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (e *ExecutionCredential) Validate() error {
	if err := ValidateIDString(e.ID); err != nil {
		return err
	}
	if err := e.RequestKey.Validate(); err != nil {
		return err
	}
	if err := ValidateIDString(e.OperationDigestID); err != nil {
		return ErrInvalidDigest
	}
	if e.Generation < 1 {
		return ErrInvalidState
	}
	switch e.State {
	case CredentialStateIssued, CredentialStateOccupied, CredentialStateCommitted, CredentialStateFailed, CredentialStateTakenOver, CredentialStateExpired:
	default:
		return ErrInvalidState
	}
	return nil
}

func (e *ExecutionCredential) CanTransitionTo(next CredentialState) bool {
	return ValidateCredentialTransition(e.State, next) == nil
}

func (e *ExecutionCredential) TransitionTo(next CredentialState, now time.Time) error {
	if err := ValidateCredentialTransition(e.State, next); err != nil {
		return err
	}
	e.State = next
	e.UpdatedAt = now
	return nil
}

func (e *ExecutionCredential) Clone() *ExecutionCredential {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}
