package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/domain"
)

func TestBug03LateCommitAfterTakeover(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	svc := application.NewService(func() time.Time { return clock })

	registered, err := svc.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "worker-a", NamespaceID: "billing", Key: "settlement-03", Digest: "digest-03",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	first, err := svc.Occupy(ctx, application.OccupyRequest{RequestID: registered.RequestID, ExpectedVersion: registered.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatalf("first occupy: %v", err)
	}
	clock = clock.Add(2 * time.Minute)
	taken, err := svc.TakeoverExpired(ctx, application.TakeoverRequest{RequestID: registered.RequestID, ExpectedVersion: first.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatalf("takeover: %v", err)
	}
	currentBefore, ok, err := svc.CredentialAt(ctx, registered.RequestID, clock)
	if err != nil || !ok {
		t.Fatalf("current credential: ok=%v err=%v", ok, err)
	}
	if currentBefore.Generation != taken.Generation {
		t.Fatalf("takeover generation = %d, want %d", currentBefore.Generation, taken.Generation)
	}

	newCommit, err := svc.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: taken.CredentialID, Generation: taken.Generation,
		ExpectedVersion: taken.Version, Payload: map[string]any{"source": "new"},
	})
	if err != nil {
		t.Fatalf("new generation commit: %v", err)
	}
	versionAfterCommit := newCommit.Version
	late, err := svc.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: first.CredentialID, Generation: first.Generation,
		ExpectedVersion: versionAfterCommit, Payload: map[string]any{"source": "old"},
	})
	if err == nil {
		t.Fatalf("late old generation committed: %+v", late)
	}
	if !errors.Is(err, domain.ErrLateCommit) {
		t.Fatalf("late commit error = %v, want domain.ErrLateCommit", err)
	}

	state, err := svc.QueryRequests(ctx, application.QueryRequest{Key: "settlement-03"})
	if err != nil {
		t.Fatalf("query after late commit: %v", err)
	}
	if len(state.Items) != 1 || state.Items[0].Version != versionAfterCommit || state.Items[0].Status != application.StatusCommitted || state.Items[0].ResultID != newCommit.ResultID {
		t.Fatalf("late commit changed state: items=%+v", state.Items)
	}
	replayed, err := svc.Replay(ctx, application.ReplayRequest{RequestID: registered.RequestID})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replayed.Payload["source"] != "new" || replayed.Generation != taken.Generation {
		t.Fatalf("replay was contaminated by old generation: generation=%d payload=%v", replayed.Generation, replayed.Payload)
	}
}
