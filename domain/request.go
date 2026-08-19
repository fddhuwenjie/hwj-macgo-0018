package domain

import "time"

type RequestState string

const (
	RequestStatePending   RequestState = "pending"
	RequestStateOccupied  RequestState = "occupied"
	RequestStateCommitted RequestState = "committed"
	RequestStateFailed    RequestState = "failed"
	RequestStateTakenOver RequestState = "taken_over"
	RequestStateExpired   RequestState = "expired"
)

type Request struct {
	ID                string
	RequestKey        RequestKey
	OperationDigestID string
	CredentialID      string
	State             RequestState
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewRequest(id string, key RequestKey, digestID string, now time.Time) (*Request, error) {
	if err := ValidateIDString(id); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := ValidateIDString(digestID); err != nil {
		return nil, ErrInvalidDigest
	}
	return &Request{
		ID:                id,
		RequestKey:        key,
		OperationDigestID: digestID,
		State:             RequestStatePending,
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (r *Request) Validate() error {
	if err := ValidateIDString(r.ID); err != nil {
		return err
	}
	if err := r.RequestKey.Validate(); err != nil {
		return err
	}
	if err := ValidateIDString(r.OperationDigestID); err != nil {
		return ErrInvalidDigest
	}
	switch r.State {
	case RequestStatePending, RequestStateOccupied, RequestStateCommitted, RequestStateFailed, RequestStateTakenOver, RequestStateExpired:
	default:
		return ErrInvalidState
	}
	return nil
}

func (r *Request) CanTransitionTo(next RequestState) bool {
	return ValidateRequestTransition(r.State, next) == nil
}

func (r *Request) TransitionTo(next RequestState, now time.Time) error {
	if err := ValidateRequestTransition(r.State, next); err != nil {
		return err
	}
	r.State = next
	r.Version++
	r.UpdatedAt = now
	if r.Version < 1 {
		r.Version = 1
	}
	return nil
}

func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}
