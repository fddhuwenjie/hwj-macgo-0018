package domain

import (
	"strings"
	"time"
)

type Namespace struct {
	ID        string
	Name      string
	Owner     string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewNamespace(id, name, owner string, now time.Time) (*Namespace, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidNamespace
	}
	if len(name) > 128 {
		return nil, ErrInvalidNamespace
	}
	if owner == "" {
		owner = id
	}
	if err := ValidateIDString(owner); err != nil {
		return nil, ErrInvalidNamespace
	}
	return &Namespace{
		ID:        id,
		Name:      name,
		Owner:     owner,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (n *Namespace) Validate() error {
	if err := ValidateIDString(n.ID); err != nil {
		return ErrInvalidNamespace
	}
	if strings.TrimSpace(n.Name) == "" {
		return ErrInvalidNamespace
	}
	if len(n.Name) > 128 {
		return ErrInvalidNamespace
	}
	if n.Owner == "" {
		return ErrInvalidNamespace
	}
	if err := ValidateIDString(n.Owner); err != nil {
		return ErrInvalidNamespace
	}
	return nil
}

func (n *Namespace) Clone() *Namespace {
	if n == nil {
		return nil
	}
	cp := *n
	return &cp
}
