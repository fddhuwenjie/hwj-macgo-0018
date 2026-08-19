package domain

import "time"

type Lease struct {
	ID           string
	CredentialID string
	Holder       string
	ExpiresAt    time.Time
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewLease(id, credentialID, holder string, expiresAt, now time.Time) (*Lease, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := ValidateIDString(credentialID); err != nil {
		return nil, ErrInvalidLease
	}
	if holder == "" {
		return nil, ErrInvalidLease
	}
	if !expiresAt.After(now) {
		return nil, ErrInvalidLease
	}
	return &Lease{
		ID:           id,
		CredentialID: credentialID,
		Holder:       holder,
		ExpiresAt:    expiresAt,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (l *Lease) Validate() error {
	if err := ValidateIDString(l.ID); err != nil {
		return err
	}
	if err := ValidateIDString(l.CredentialID); err != nil {
		return ErrInvalidLease
	}
	if l.Holder == "" {
		return ErrInvalidLease
	}
	if l.ExpiresAt.IsZero() {
		return ErrInvalidLease
	}
	return nil
}

func (l *Lease) IsExpired(now time.Time) bool {
	return now.Before(l.ExpiresAt)
}

func (l *Lease) Clone() *Lease {
	if l == nil {
		return nil
	}
	cp := *l
	return &cp
}
