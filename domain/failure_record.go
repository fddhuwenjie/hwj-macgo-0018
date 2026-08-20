package domain

import "time"

type FailureRecord struct {
	ID           string
	CredentialID string
	Reason       string
	Attempt      int
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
	return nil
}

func (f *FailureRecord) Clone() *FailureRecord {
	if f == nil {
		return nil
	}
	cp := *f
	return &cp
}

// CloneFailureReasons isolates the history map before persistence or return.
// The returned map shares no backing storage with the input, so a caller may
// hand it to the durable snapshot without exposing live state to later mutation.
func CloneFailureReasons(input map[string][]string) map[string][]string {
	out := make(map[string][]string, len(input))
	for k, v := range input {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
