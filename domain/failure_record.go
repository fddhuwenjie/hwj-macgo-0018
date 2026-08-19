package domain

import "time"

type FailureRecord struct {
	ID           string
	CredentialID string
	Reason       string
	Attempt      int
	Generation   uint64
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewFailureRecord(id, credentialID, reason string, attempt int, now time.Time) (*FailureRecord, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := ValidateIDString(credentialID); err != nil {
		return nil, ErrNoCredential
	}
	if reason == "" {
		return nil, ErrInvalidState
	}
	if attempt < 1 {
		return nil, ErrInvalidState
	}
	return &FailureRecord{
		ID:           id,
		CredentialID: credentialID,
		Reason:       reason,
		Attempt:      attempt,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NewFailureRecordAtGeneration records the execution generation at the write boundary.
func NewFailureRecordAtGeneration(id, credentialID, reason string, attempt int, generation uint64, now time.Time) (*FailureRecord, error) {
	record, err := NewFailureRecord(id, credentialID, reason, attempt, now)
	if err != nil {
		return nil, err
	}
	record.Generation = generation
	return record, nil
}

func (f *FailureRecord) Validate() error {
	if err := ValidateIDString(f.ID); err != nil {
		return err
	}
	if err := ValidateIDString(f.CredentialID); err != nil {
		return ErrNoCredential
	}
	if f.Reason == "" {
		return ErrInvalidState
	}
	if f.Attempt < 1 {
		return ErrInvalidState
	}
	// Generation zero is the initial generation and remains valid for legacy records.
	return nil
}

func (f *FailureRecord) Clone() *FailureRecord {
	if f == nil {
		return nil
	}
	cp := *f
	return &cp
}
