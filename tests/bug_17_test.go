package tests

import (
	"context"
	"hwj-macgo-0018/application"
	"testing"
	"time"
)

func TestBug17ReplayUsesCurrentGeneration(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1700000000, 0)
	s := application.NewService(func() time.Time { return now })
	pre, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "g", NamespaceID: "n", Key: "generation", Digest: "d", LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	occ, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: occ.Generation, ExpectedVersion: occ.Version, Payload: map[string]any{"generation": float64(1)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareGenerationForReplay(pre.RequestID); err != nil {
		t.Fatal(err)
	}
	current, err := s.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: current.Generation, ExpectedVersion: current.Version, Payload: map[string]any{"generation": float64(2)}}); err != nil {
		t.Fatal(err)
	}
	replay, err := s.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if replay.Generation != current.Generation || replay.Payload["generation"] != float64(2) {
		t.Fatalf("replay returned stale generation: %#v", replay)
	}
}
