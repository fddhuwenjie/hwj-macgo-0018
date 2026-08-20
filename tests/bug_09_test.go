package tests

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/domain"
)

type bug09Persister struct {
	mu   sync.Mutex
	data map[string]any
}

func (p *bug09Persister) Load() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneBug09Data(p.data), nil
}

func (p *bug09Persister) Save(data map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = cloneBug09Data(data)
	return nil
}

func cloneBug09Data(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func bug09NestedState(payload map[string]any) any {
	nested, ok := payload["nested"].(map[string]any)
	if !ok {
		return nil
	}
	return nested["state"]
}

func TestBug09ResultPayloadIsolationDiagnosis(t *testing.T) {
	ctx := context.Background()
	store := &bug09Persister{}
	service := application.NewService(store)
	pre, err := service.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "caller", NamespaceID: "ns", Key: "payload", Digest: "digest",
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
	payload := map[string]any{"nested": map[string]any{"state": "original"}}
	committed, err := service.Commit(ctx, application.CommitRequest{
		RequestID:       pre.RequestID,
		CredentialID:    occupied.CredentialID,
		Generation:      occupied.Generation,
		ExpectedVersion: occupied.Version,
		Payload:         payload,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload["nested"].(map[string]any)["state"] = "changed-by-caller"
	firstReplay, err := service.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if got := bug09NestedState(firstReplay.Payload); got != "original" {
		t.Errorf("caller input mutated committed history: got %v", got)
	}

	firstReplay.Payload["nested"].(map[string]any)["state"] = "changed-by-replay-caller"
	secondReplay, err := service.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if got := bug09NestedState(secondReplay.Payload); got != "original" {
		t.Errorf("returned replay mutated in-memory history: got %v", got)
	}
	if secondReplay.ResultID != committed.ResultID || secondReplay.Generation != occupied.Generation {
		t.Errorf("legal replay lost result identity: %#v", secondReplay)
	}

	reopened := application.NewService(store)
	afterReopen, err := reopened.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if got := bug09NestedState(afterReopen.Payload); got != "original" {
		t.Errorf("reopen retained a caller-side mutation: got %v", got)
	}
	request, err := reopened.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if request.Status != application.StatusCommitted || request.ResultID != committed.ResultID {
		t.Errorf("follow-up replay changed the committed request: %#v", request)
	}

	snapshot, err := domain.NewResultSnapshot("result-1", "credential-1", "digest-1", []byte("original"), time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	cloned := snapshot.Clone()
	cloned.Payload[0] = 'X'
	if got := string(snapshot.Payload); got != "original" {
		t.Errorf("domain snapshot clone shares payload storage: got %q", got)
	}
}
