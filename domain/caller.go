package domain

import (
	"strings"
	"time"
)

type Caller struct {
	ID        string
	Name      string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewCaller(id, name string, now time.Time) (*Caller, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidCaller
	}
	if len(name) > 128 {
		return nil, ErrInvalidCaller
	}
	return &Caller{
		ID:        id,
		Name:      name,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c *Caller) Validate() error {
	if err := ValidateIDString(c.ID); err != nil {
		return ErrInvalidCaller
	}
	if strings.TrimSpace(c.Name) == "" {
		return ErrInvalidCaller
	}
	if len(c.Name) > 128 {
		return ErrInvalidCaller
	}
	return nil
}

func (c *Caller) Clone() *Caller {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}
