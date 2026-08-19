package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// flakyPersister succeeds until the failAt-th Save call, then fails every Save
// after it. It lets a test drive the persist-failure rollback path of Occupy.
type flakyPersister struct {
	mu     sync.Mutex
	failAt int
	calls  int
}

func (f *flakyPersister) Load() (map[string]any, error) { return nil, nil }

func (f *flakyPersister) Save(map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls >= f.failAt {
		return errors.New("persist unavailable")
	}
	return nil
}

// countCredentialsForRequest counts how many credentials currently reference a
// request. After a clean takeover there is exactly one; an orphaned credential
// left behind by a failed competitor would push the count above one.
func countCredentialsForRequest(s *UseCase, requestID string) int {
	n := 0
	for _, c := range s.credentials {
		if c.RequestID == requestID {
			n++
		}
	}
	return n
}

// TestOccupyFreshRollsBackOnPersistFailure drives the fresh-occupy branch
// (PreRegistered -> Occupied) through a persist failure and asserts that the
// request is restored and no credential is left behind.
func TestOccupyFreshRollsBackOnPersistFailure(t *testing.T) {
	clock := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
	// PreRegister persists (call 1); the fresh Occupy persists (call 2) and fails.
	p := &flakyPersister{failAt: 2}
	s := NewService(p, func() time.Time { return clock })
	ctx := context.Background()

	pr, err := s.PreRegister(ctx, PreRegisterRequest{CallerID: "rb", NamespaceID: "lease", Key: "fresh-07", Digest: "d07", LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.FindRequest(ctx, pr.RequestID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Occupy(ctx, OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: time.Minute}); err == nil {
		t.Fatal("expected occupy to fail when persist fails")
	}

	got, err := s.FindRequest(ctx, pr.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CredentialID != before.CredentialID {
		t.Fatalf("credential not rolled back: got %q want %q", got.CredentialID, before.CredentialID)
	}
	if got.Generation != before.Generation {
		t.Fatalf("generation not rolled back: got %d want %d", got.Generation, before.Generation)
	}
	if got.Version != before.Version {
		t.Fatalf("version not rolled back: got %d want %d", got.Version, before.Version)
	}
	if got.Status != before.Status {
		t.Fatalf("status not rolled back: got %q want %q", got.Status, before.Status)
	}
	if n := countCredentialsForRequest(s, pr.RequestID); n != 0 {
		t.Fatalf("expected 0 credentials for request after rollback, got %d", n)
	}
}

// TestOccupyReoccupyRollsBackOnPersistFailure drives the expired-lease takeover
// branch through a persist failure and asserts that the request keeps its prior
// generation/credential and that the contender's freshly minted credential is
// removed rather than left as an extra active credential.
func TestOccupyReoccupyRollsBackOnPersistFailure(t *testing.T) {
	clock := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
	// PreRegister (1), first Occupy (2), expired-lease re-occupy (3) fails.
	p := &flakyPersister{failAt: 3}
	s := NewService(p, func() time.Time { return clock })
	ctx := context.Background()

	pr, err := s.PreRegister(ctx, PreRegisterRequest{CallerID: "rb", NamespaceID: "lease", Key: "reoccupy-07", Digest: "d07", LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Occupy(ctx, OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	// Let the lease lapse, then attempt the takeover whose persist fails.
	clock = clock.Add(2 * time.Minute)
	if _, err := s.Occupy(ctx, OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: first.Version, LeaseTTL: time.Minute}); err == nil {
		t.Fatal("expected re-occupy to fail when persist fails")
	}

	got, err := s.FindRequest(ctx, pr.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CredentialID != first.CredentialID {
		t.Fatalf("credential not rolled back: got %q want %q", got.CredentialID, first.CredentialID)
	}
	if got.Generation != first.Generation {
		t.Fatalf("generation not rolled back: got %d want %d", got.Generation, first.Generation)
	}
	if got.Version != first.Version {
		t.Fatalf("version not rolled back: got %d want %d", got.Version, first.Version)
	}
	if got.Status != StatusOccupied {
		t.Fatalf("status not rolled back: got %q want %q", got.Status, StatusOccupied)
	}
	// The original credential must still be the only one for this request: the
	// failed contender's credential must not survive as an extra active credential.
	orig := s.credentials[got.CredentialID]
	if orig == nil {
		t.Fatal("original credential missing after rollback")
	}
	if orig.Status != credentialActive {
		t.Fatalf("original credential status not restored: got %q want %q", orig.Status, credentialActive)
	}
	if n := countCredentialsForRequest(s, pr.RequestID); n != 1 {
		t.Fatalf("expected exactly 1 credential for request after rollback, got %d", n)
	}
}
