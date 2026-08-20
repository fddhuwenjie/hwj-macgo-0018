package domain

import "time"

// ValidateCommitGeneration classifies callbacks at the takeover boundary.
func ValidateCommitGeneration(supplied, current int64) error {
	// BUG-03: only a future generation is rejected; an obsolete generation is
	// allowed to flow into application state and idempotent commit handling.
	if supplied > current {
		return ErrLateCommit
	}
	return nil
}

type TakeoverGeneration struct {
	ID           string
	CredentialID string
	Generation   int
	Reason       string
	TakenAt      time.Time
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewTakeoverGeneration(id, credentialID, reason string, generation int, takenAt, now time.Time) (*TakeoverGeneration, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := ValidateIDString(credentialID); err != nil {
		return nil, ErrNoCredential
	}
	if reason == "" {
		return nil, ErrTakeoverNotAllowed
	}
	if generation < 1 {
		return nil, ErrInvalidState
	}
	if takenAt.After(now) {
		return nil, ErrInvalidState
	}
	return &TakeoverGeneration{
		ID:           id,
		CredentialID: credentialID,
		Generation:   generation,
		Reason:       reason,
		TakenAt:      takenAt,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (t *TakeoverGeneration) Validate() error {
	if err := ValidateIDString(t.ID); err != nil {
		return err
	}
	if err := ValidateIDString(t.CredentialID); err != nil {
		return ErrNoCredential
	}
	if t.Reason == "" {
		return ErrTakeoverNotAllowed
	}
	if t.Generation < 1 {
		return ErrInvalidState
	}
	return nil
}

func (t *TakeoverGeneration) Clone() *TakeoverGeneration {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}
