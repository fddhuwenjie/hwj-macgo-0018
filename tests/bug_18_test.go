package tests

import (
	"context"
	"encoding/json"
	"hwj-macgo-0018/application"
	"sync"
	"testing"
)

type bug18Persister struct {
	mu   sync.Mutex
	data map[string]any
}

func (p *bug18Persister) Load() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return bug18Clone(p.data), nil
}
func (p *bug18Persister) Save(data map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = bug18Clone(data)
	return nil
}
func bug18Clone(v map[string]any) map[string]any {
	b, _ := json.Marshal(v)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func TestBug18FailureHistorySurvivesReopenDiagnosis(t *testing.T) {
	ctx := context.Background()
	p := &bug18Persister{}
	s := application.NewService(p)
	pre, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "ops", NamespaceID: "recovery", Key: "failure-history", Digest: "d"})
	if err != nil {
		t.Fatal(err)
	}
	occ, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.FailRelease(ctx, application.FailReleaseRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: occ.Generation, ExpectedVersion: occ.Version, Reason: "worker stopped before acknowledgement"}); err != nil {
		t.Fatal(err)
	}
	history, err := s.FailureHistory(ctx, pre.RequestID)
	if err != nil || len(history) != 1 {
		t.Fatalf("in-memory failure history missing: %#v %v", history, err)
	}
	reopened := application.NewService(p)
	durable, err := reopened.FailureHistory(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(durable) != 1 || durable[0] != history[0] {
		t.Fatalf("failure history disappeared after reopen: %#v", durable)
	}
}

// TestBug18LegitimateRetryAfterReopen locks in the full user scenario: after a
// service reopen, the failure reasons and the released status survive together,
// and a caller can still drive one legal retry from that durable history.
func TestBug18LegitimateRetryAfterReopen(t *testing.T) {
	ctx := context.Background()
	p := &bug18Persister{}
	s := application.NewService(p)
	pre, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "ops", NamespaceID: "retry", Key: "legit-retry", Digest: "d"})
	if err != nil {
		t.Fatal(err)
	}
	occ, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.FailRelease(ctx, application.FailReleaseRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: occ.Generation, ExpectedVersion: occ.Version, Reason: "downstream timeout"}); err != nil {
		t.Fatal(err)
	}

	// Reopen: the service process is gone and comes back against the same store.
	reopened := application.NewService(p)

	// The judgment basis — prior failure reasons — must survive the reopen.
	prior, err := reopened.FailureHistory(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prior) != 1 || prior[0] != "downstream timeout" {
		t.Errorf("reopen lost the retry judgment basis: %#v", prior)
	}
	// The released status and last reason must travel with the request snapshot.
	req, err := reopened.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != application.StatusReleased || req.FailureReason != "downstream timeout" {
		t.Fatalf("reopen lost released status/reason: status=%q reason=%q", req.Status, req.FailureReason)
	}

	// A legal retry: re-occupy from the released state at the durable version.
	retry, err := reopened.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: req.Version})
	if err != nil {
		t.Fatalf("legal retry after reopen failed: %v", err)
	}
	if retry.CredentialID == "" || retry.CredentialID == occ.CredentialID {
		t.Fatalf("retry did not mint a fresh credential: %#v", retry)
	}
	if retry.Generation != occ.Generation+1 || retry.Status != application.StatusOccupied {
		t.Fatalf("retry did not advance generation/status: %#v", retry)
	}

	// A second release on the retry adds a second reason; history accumulates
	// across the reopen so the next retry still has the full basis.
	if _, err := reopened.FailRelease(ctx, application.FailReleaseRequest{RequestID: pre.RequestID, CredentialID: retry.CredentialID, Generation: retry.Generation, ExpectedVersion: retry.Version, Reason: "quota exceeded"}); err != nil {
		t.Fatal(err)
	}
	reopened2 := application.NewService(p)
	accumulated, err := reopened2.FailureHistory(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accumulated) != 2 || accumulated[0] != "downstream timeout" || accumulated[1] != "quota exceeded" {
		t.Errorf("failure history did not accumulate across reopens: %#v", accumulated)
	}
}
