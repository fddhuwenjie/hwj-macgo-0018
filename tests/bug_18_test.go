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
