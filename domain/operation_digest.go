package domain

import (
	"time"
)

type OperationDigest struct {
	ID         string
	RequestKey RequestKey
	Algorithm  string
	Hash       string
	Summary    string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewOperationDigest(id string, key RequestKey, algorithm, hash, summary string, now time.Time) (*OperationDigest, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateAlgorithm(algorithm); err != nil {
		return nil, err
	}
	if err := ValidateDigestHash(hash); err != nil {
		return nil, err
	}
	return &OperationDigest{
		ID:         id,
		RequestKey: key,
		Algorithm:  algorithm,
		Hash:       hash,
		Summary:    summary,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (o *OperationDigest) Validate() error {
	if err := ValidateIDString(o.ID); err != nil {
		return err
	}
	if err := o.RequestKey.Validate(); err != nil {
		return err
	}
	if err := ValidateAlgorithm(o.Algorithm); err != nil {
		return err
	}
	if err := ValidateDigestHash(o.Hash); err != nil {
		return err
	}
	return nil
}

func (o *OperationDigest) Clone() *OperationDigest {
	if o == nil {
		return nil
	}
	cp := *o
	return &cp
}
