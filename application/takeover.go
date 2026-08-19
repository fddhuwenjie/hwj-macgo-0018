package application

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type TakeoverRequest struct {
	RequestID       string
	ExpectedVersion int64
	LeaseTTL        time.Duration
}

type TakeoverResponse struct {
	RequestID       string
	CredentialID    string
	OldCredentialID string
	Generation      int64
	Version         int64
	ExpiresAt       time.Time
}

func (s *UseCase) TakeoverExpired(ctx context.Context, in TakeoverRequest) (TakeoverResponse, error) {
	if err := s.checkContext(ctx); err != nil {
		return TakeoverResponse{}, err
	}
	if blank(in.RequestID) || in.LeaseTTL <= 0 {
		return TakeoverResponse{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[in.RequestID]
	if !ok {
		return TakeoverResponse{}, ErrNotFound
	}
	if req.Version != in.ExpectedVersion {
		return TakeoverResponse{}, ErrOptimisticLock
	}
	if req.Status != StatusOccupied || blank(req.CredentialID) {
		return TakeoverResponse{}, ErrIllegalTransition
	}
	old, ok := s.credentials[req.CredentialID]
	if !ok {
		return TakeoverResponse{}, ErrNotFound
	}
	now := s.nowFunc()
	if now.Before(old.ExpiresAt) {
		return TakeoverResponse{}, ErrConflict
	}

	requestBefore := *req
	credentialBefore := *old
	old.Status = credentialExpired
	old.Version++
	old.UpdatedAt = now

	newCredential := &Credential{
		ID:         s.newID("credential"),
		RequestID:  req.ID,
		Generation: req.Generation + 1,
		Status:     credentialActive,
		Version:    1,
		ExpiresAt:  now.Add(in.LeaseTTL),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.credentials[newCredential.ID] = newCredential
	req.CredentialID = newCredential.ID
	req.Generation = newCredential.Generation
	req.LeaseExpiresAt = newCredential.ExpiresAt
	req.Version++
	req.UpdatedAt = now
	if err := s.persist(); err != nil {
		*req = requestBefore
		*old = credentialBefore
		delete(s.credentials, newCredential.ID)
		return TakeoverResponse{}, fmt.Errorf("takeover persist: %w", ErrStoreFailed)
	}
	return TakeoverResponse{
		RequestID:       req.ID,
		CredentialID:    newCredential.ID,
		OldCredentialID: old.ID,
		Generation:      newCredential.Generation,
		Version:         req.Version,
		ExpiresAt:       newCredential.ExpiresAt,
	}, nil
}

type ExpirationItem struct {
	RequestID    string
	CredentialID string
	Generation   int64
	ExpiredAt    time.Time
	Version      int64
}

type ExpirationBatch struct {
	Scanned int
	Expired []ExpirationItem
}

func (s *UseCase) ExpireCredentials(ctx context.Context, at time.Time, limit int) (ExpirationBatch, error) {
	if err := s.checkContext(ctx); err != nil {
		return ExpirationBatch{}, err
	}
	if at.IsZero() || limit < 0 {
		return ExpirationBatch{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.requests))
	for id := range s.requests {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	batch := ExpirationBatch{Expired: make([]ExpirationItem, 0)}
	requestBackups := make(map[string]Request)
	credentialBackups := make(map[string]Credential)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			restoreExpirationState(s, requestBackups, credentialBackups)
			return ExpirationBatch{}, err
		}
		if limit > 0 && len(batch.Expired) >= limit {
			break
		}
		batch.Scanned++
		req := s.requests[id]
		if req == nil || req.Status != StatusOccupied || req.CredentialID == "" {
			continue
		}
		credential := s.credentials[req.CredentialID]
		if credential == nil || credential.Status != credentialActive || at.Before(credential.ExpiresAt) {
			continue
		}
		requestBackups[id] = *req
		credentialBackups[credential.ID] = *credential
		credential.Status = credentialExpired
		credential.Version++
		credential.UpdatedAt = at
		req.Status = StatusFailed
		req.FailureReason = "execution lease expired"
		req.Version++
		req.UpdatedAt = at
		batch.Expired = append(batch.Expired, ExpirationItem{
			RequestID:    req.ID,
			CredentialID: credential.ID,
			Generation:   credential.Generation,
			ExpiredAt:    credential.ExpiresAt,
			Version:      req.Version,
		})
	}
	if len(batch.Expired) == 0 {
		return batch, nil
	}
	if err := s.persist(); err != nil {
		restoreExpirationState(s, requestBackups, credentialBackups)
		return ExpirationBatch{}, fmt.Errorf("expiration batch persist: %w", ErrStoreFailed)
	}
	return batch, nil
}

func restoreExpirationState(s *UseCase, requests map[string]Request, credentials map[string]Credential) {
	for id, before := range requests {
		if current := s.requests[id]; current != nil {
			*current = before
		}
	}
	for id, before := range credentials {
		if current := s.credentials[id]; current != nil {
			*current = before
		}
	}
}

func (s *UseCase) CredentialAt(ctx context.Context, requestID string, at time.Time) (Credential, bool, error) {
	if err := s.checkContext(ctx); err != nil {
		return Credential{}, false, err
	}
	if blank(requestID) || at.IsZero() {
		return Credential{}, false, ErrInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[requestID]
	if !ok {
		return Credential{}, false, ErrNotFound
	}
	credential, ok := s.credentials[req.CredentialID]
	if !ok || credential.Status != credentialActive {
		return Credential{}, false, nil
	}
	if at.Before(credential.CreatedAt) || !at.Before(credential.ExpiresAt) {
		return Credential{}, false, nil
	}
	return *credential, true, nil
}
