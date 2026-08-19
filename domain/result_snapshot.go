package domain

import "time"

type ResultSnapshot struct {
	ID           string
	CredentialID string
	Digest       string
	Payload      []byte
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewResultSnapshot(id, credentialID, digest string, payload []byte, now time.Time) (*ResultSnapshot, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := ValidateIDString(credentialID); err != nil {
		return nil, ErrNoCredential
	}
	if err := ValidateDigestHash(digest); err != nil {
		return nil, err
	}
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	return &ResultSnapshot{
		ID:           id,
		CredentialID: credentialID,
		Digest:       digest,
		Payload:      payloadCopy,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *ResultSnapshot) Validate() error {
	if err := ValidateIDString(r.ID); err != nil {
		return err
	}
	if err := ValidateIDString(r.CredentialID); err != nil {
		return ErrNoCredential
	}
	if err := ValidateDigestHash(r.Digest); err != nil {
		return err
	}
	return nil
}

func (r *ResultSnapshot) Clone() *ResultSnapshot {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}
