package tests

import (
	"context"
	"hwj-macgo-0018/application"
	"testing"
)

func TestBug09ResultPayloadIsolationDiagnosis(t *testing.T) {
	ctx := context.Background()
	s := application.NewService()
	pre, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "caller", NamespaceID: "ns", Key: "payload", Digest: "digest"})
	if err != nil {
		t.Fatal(err)
	}
	occ, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"nested": map[string]any{"state": "original"}}
	if _, err := s.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: occ.Generation, ExpectedVersion: occ.Version, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	payload["nested"].(map[string]any)["state"] = "changed-by-caller"
	replayed, err := s.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if got := replayed.Payload["nested"].(map[string]any)["state"]; got != "original" {
		t.Fatalf("history was mutated through caller payload: %#v", replayed.Payload)
	}
}
