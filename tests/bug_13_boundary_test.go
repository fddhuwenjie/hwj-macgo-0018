package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
)

// noVersionCheck disables the optimistic-version guard on entry points that
// support opting out (ExpectedVersion < 0 means "do not check"). The boundary
// tests exercise expiry, not concurrency, so they opt out of version checks.
const noVersionCheck = -1

// mutableClock feeds a controllable time into an application service so the
// expiry boundary can be exercised without sleeping.
type mutableClock struct{ now time.Time }

func (m *mutableClock) Now() time.Time { return m.now }

func newBoundaryService(t *testing.T, start time.Time) (*application.UseCase, *mutableClock) {
	t.Helper()
	clk := &mutableClock{now: start}
	return application.NewService(func() time.Time { return clk.Now() }), clk
}

// preRegisterAndOccupy runs the happy path up to an occupied lease and returns
// the request id, credential id and generation.
func preRegisterAndOccupy(t *testing.T, svc *application.UseCase, key string, ttl time.Duration) (string, string, int64) {
	t.Helper()
	pr, err := svc.PreRegister(context.Background(), application.PreRegisterRequest{
		CallerID: "c", NamespaceID: "n", Key: key, Digest: "d", LeaseTTL: ttl,
	})
	if err != nil {
		t.Fatalf("pre-register: %v", err)
	}
	oc, err := svc.Occupy(context.Background(), application.OccupyRequest{
		RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: ttl,
	})
	if err != nil {
		t.Fatalf("occupy: %v", err)
	}
	return pr.RequestID, oc.CredentialID, oc.Generation
}

// TestBoundaryValidLeaseIsUsableBeforeDeadline covers the user's invariant: a
// lease before its deadline is always usable, so operations on it succeed.
func TestBoundaryValidLeaseIsUsableBeforeDeadline(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	svc, clk := newBoundaryService(t, t0)
	reqID, credID, gen := preRegisterAndOccupy(t, svc, "usable", time.Hour)

	// Re-occupy 30 minutes before the deadline must renew nothing: the existing
	// credential is still returned unchanged.
	clk.now = t0.Add(30 * time.Minute)
	oc, err := svc.Occupy(context.Background(), application.OccupyRequest{
		RequestID: reqID, ExpectedVersion: noVersionCheck, LeaseTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("re-occupy before deadline: %v", err)
	}
	if oc.CredentialID != credID {
		t.Fatalf("valid lease was renewed: credential changed from %s to %s", credID, oc.CredentialID)
	}

	// A commit well before the deadline succeeds.
	if _, err := svc.Commit(context.Background(), application.CommitRequest{
		RequestID: reqID, CredentialID: credID, Generation: gen, ExpectedVersion: noVersionCheck,
	}); err != nil {
		t.Fatalf("commit before deadline: %v", err)
	}
}

// TestBoundaryTakeoverAllowedAtDeadline verifies the takeover entry point treats
// the deadline instant as expired (convention: now >= ExpiresAt).
func TestBoundaryTakeoverAllowedAtDeadline(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	svc, clk := newBoundaryService(t, t0)
	reqID, _, _ := preRegisterAndOccupy(t, svc, "takeover", time.Hour)

	// Takeover requires the current request version (no opt-out).
	clk.now = t0.Add(time.Hour)
	req, err := svc.FindRequest(context.Background(), reqID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TakeoverExpired(context.Background(), application.TakeoverRequest{
		RequestID: reqID, ExpectedVersion: req.Version, LeaseTTL: time.Hour,
	}); err != nil {
		t.Fatalf("takeover at deadline should be allowed, got: %v", err)
	}
}

// TestBoundaryCommitRejectedAtDeadline verifies the commit entry point agrees:
// at the deadline the lease is expired and the commit is rejected, without a
// takeover having run first (the credential is still active in state).
func TestBoundaryCommitRejectedAtDeadline(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	svc, clk := newBoundaryService(t, t0)
	reqID, credID, gen := preRegisterAndOccupy(t, svc, "commit-deadline", time.Hour)

	clk.now = t0.Add(time.Hour)
	if _, err := svc.Commit(context.Background(), application.CommitRequest{
		RequestID: reqID, CredentialID: credID, Generation: gen, ExpectedVersion: noVersionCheck,
	}); err != application.ErrExpired {
		t.Fatalf("commit at deadline: expected ErrExpired, got %v", err)
	}
}

// TestBoundaryFailedCheckDoesNotMutateLease is the core guarantee requested: a
// single failed (rejecting) expiry check must not modify the lease's state or
// version. Commit's expiry guard rejects an expired credential without touching
// the credential or the request.
func TestBoundaryFailedCheckDoesNotMutateLease(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	svc, clk := newBoundaryService(t, t0)
	reqID, credID, gen := preRegisterAndOccupy(t, svc, "no-mutate", time.Hour)

	// Snapshot the request and credential before the failed check.
	reqBefore, err := svc.FindRequest(context.Background(), reqID)
	if err != nil {
		t.Fatal(err)
	}
	credBefore, err := svc.FindCredential(context.Background(), credID)
	if err != nil {
		t.Fatal(err)
	}

	// Advance past the deadline and attempt a commit, which the expiry guard rejects.
	clk.now = t0.Add(2 * time.Hour)
	if _, err := svc.Commit(context.Background(), application.CommitRequest{
		RequestID: reqID, CredentialID: credID, Generation: gen, ExpectedVersion: noVersionCheck,
	}); err != application.ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}

	// Neither the request nor the credential may have changed state or version.
	reqAfter, err := svc.FindRequest(context.Background(), reqID)
	if err != nil {
		t.Fatal(err)
	}
	if reqAfter.Status != reqBefore.Status {
		t.Fatalf("failed check mutated request status: %q -> %q", reqBefore.Status, reqAfter.Status)
	}
	if reqAfter.Version != reqBefore.Version {
		t.Fatalf("failed check mutated request version: %d -> %d", reqBefore.Version, reqAfter.Version)
	}
	credAfter, err := svc.FindCredential(context.Background(), credID)
	if err != nil {
		t.Fatal(err)
	}
	if credAfter.Status != credBefore.Status {
		t.Fatalf("failed check mutated credential status: %q -> %q", credBefore.Status, credAfter.Status)
	}
	if credAfter.Version != credBefore.Version {
		t.Fatalf("failed check mutated credential version: %d -> %d", credBefore.Version, credAfter.Version)
	}
}

// TestBoundaryScanDoesNotPrematurelyExpire covers the reported symptom that the
// background scan prematurely moves a still-valid lease to a terminal state.
// The scan must only expire leases whose deadline has been reached.
func TestBoundaryScanDoesNotPrematurelyExpire(t *testing.T) {
	t0 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	svc, clk := newBoundaryService(t, t0)
	reqID, credID, _ := preRegisterAndOccupy(t, svc, "scan", time.Hour)

	// Snapshot state before a scan that runs while the lease still has an hour left.
	reqBefore, err := svc.FindRequest(context.Background(), reqID)
	if err != nil {
		t.Fatal(err)
	}
	credBefore, err := svc.FindCredential(context.Background(), credID)
	if err != nil {
		t.Fatal(err)
	}

	// Scan while the lease still has an hour left: nothing is expired and the
	// lease is not moved to a terminal state or version-bumped.
	batch, err := svc.ExpireCredentials(context.Background(), clk.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Expired) != 0 {
		t.Fatalf("background scan prematurely expired %d valid lease(s)", len(batch.Expired))
	}
	cred, err := svc.FindCredential(context.Background(), credID)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Status != credBefore.Status || cred.Version != credBefore.Version {
		t.Fatalf("background scan mutated valid lease: status %q->%q, version %d->%d",
			credBefore.Status, cred.Status, credBefore.Version, cred.Version)
	}
	req, err := svc.FindRequest(context.Background(), reqID)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != reqBefore.Status || req.Version != reqBefore.Version {
		t.Fatalf("background scan mutated valid request: status %q->%q, version %d->%d",
			reqBefore.Status, req.Status, reqBefore.Version, req.Version)
	}

	// Once the deadline is reached, the scan expires it as expected.
	batch, err = svc.ExpireCredentials(context.Background(), t0.Add(time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Expired) != 1 {
		t.Fatalf("expected 1 expired lease at deadline, got %d", len(batch.Expired))
	}
}
