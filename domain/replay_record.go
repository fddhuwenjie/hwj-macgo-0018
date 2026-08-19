package domain

import "time"

type ReplayRecord struct {
	ID           string
	RequestKey   RequestKey
	CredentialID string
	ReplayedAt   time.Time
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewReplayRecord(id string, key RequestKey, credentialID string, replayedAt, now time.Time) (*ReplayRecord, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateIDString(credentialID); err != nil {
		return nil, ErrNoCredential
	}
	if replayedAt.After(now) {
		return nil, ErrInvalidState
	}
	return &ReplayRecord{
		ID:           id,
		RequestKey:   key,
		CredentialID: credentialID,
		ReplayedAt:   replayedAt,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *ReplayRecord) Validate() error {
	if err := ValidateIDString(r.ID); err != nil {
		return err
	}
	if err := r.RequestKey.Validate(); err != nil {
		return err
	}
	if err := ValidateIDString(r.CredentialID); err != nil {
		return ErrNoCredential
	}
	return nil
}

func (r *ReplayRecord) Clone() *ReplayRecord {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}
