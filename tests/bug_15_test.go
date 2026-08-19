package tests

import (
	"context"
	"encoding/json"
	"errors"
	"hwj-macgo-0018/application"
	"sync"
	"testing"
)

type bug15Persister struct {
	mu        sync.Mutex
	data      map[string]any
	saves     int
	blockAt   int
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func bug15Clone(data map[string]any) map[string]any {
	encoded, _ := json.Marshal(data)
	cloned := map[string]any{}
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func (p *bug15Persister) Load() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return bug15Clone(p.data), nil
}

func (p *bug15Persister) Save(data map[string]any) error {
	p.mu.Lock()
	p.saves++
	blocked := p.saves == p.blockAt
	started, release := p.started, p.release
	p.mu.Unlock()
	if blocked {
		p.startOnce.Do(func() { close(started) })
		<-release
	}
	p.mu.Lock()
	p.data = bug15Clone(data)
	p.mu.Unlock()
	return nil
}

func (p *bug15Persister) blockNextSave() (<-chan struct{}, chan<- struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockAt = p.saves + 1
	p.started = make(chan struct{})
	p.release = make(chan struct{})
	p.startOnce = sync.Once{}
	return p.started, p.release
}

func TestBug15CancelledCommitDoesNotPersistDiagnosis(t *testing.T) {
	ctx := context.Background()
	p := &bug15Persister{}
	service := application.NewService(p)
	pre, err := service.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "cancel-caller", NamespaceID: "runtime", Key: "finished-run", Digest: "payload-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := service.Occupy(ctx, application.OccupyRequest{
		RequestID: pre.RequestID, ExpectedVersion: pre.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := service.Commit(ctx, application.CommitRequest{
		RequestID: pre.RequestID, CredentialID: occupied.CredentialID,
		Generation: occupied.Generation, ExpectedVersion: occupied.Version,
		Payload: map[string]any{"artifact": "ready"},
	})
	if err != nil {
		t.Fatal(err)
	}

	started, release := p.blockNextSave()
	cancelCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		_, commitErr := service.Commit(cancelCtx, application.CommitRequest{
			RequestID: pre.RequestID, CredentialID: occupied.CredentialID,
			Generation: occupied.Generation, ExpectedVersion: committed.Version,
			Payload: map[string]any{"artifact": "ready"},
		})
		result <- commitErr
	}()
	<-started
	cancel()
	close(release)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked replay should return context cancellation: %v", err)
	}

	current, err := service.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != committed.Version+1 {
		t.Fatalf("diagnosis expected cancelled replay to advance in-memory version: got %d want %d", current.Version, committed.Version+1)
	}
	reopened := application.NewService(p)
	durable, err := reopened.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if durable.Version != committed.Version+1 {
		t.Fatalf("diagnosis expected cancelled replay to advance durable version: got %d want %d", durable.Version, committed.Version+1)
	}
	_, err = reopened.Commit(ctx, application.CommitRequest{
		RequestID: pre.RequestID, CredentialID: occupied.CredentialID,
		Generation: occupied.Generation, ExpectedVersion: committed.Version,
		Payload: map[string]any{"artifact": "ready"},
	})
	if !errors.Is(err, application.ErrOptimisticLock) {
		t.Fatalf("diagnosis expected cancellation to invalidate the prior version: %v", err)
	}
}
