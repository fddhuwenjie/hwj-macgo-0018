package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
)

func TestBug01RetryAfterFailure(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s := application.NewService(func() time.Time { return now })
	ctx := context.Background()
	registered, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "worker", NamespaceID: "media", Key: "clip-77", Digest: "d1", LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Occupy(ctx, application.OccupyRequest{RequestID: registered.RequestID, ExpectedVersion: registered.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := s.FailRelease(ctx, application.FailReleaseRequest{RequestID: first.RequestID, CredentialID: first.CredentialID, Generation: first.Generation, ExpectedVersion: first.Version, Reason: "transient decoder failure"})
	if err != nil || !failed.Released {
		t.Fatalf("release: %v %#v", err, failed)
	}
	second, err := s.Occupy(ctx, application.OccupyRequest{RequestID: registered.RequestID, ExpectedVersion: failed.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.FindRequest(ctx, registered.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Generation != first.Generation+1 {
		t.Errorf("retry reused generation: first=%d second=%d", first.Generation, second.Generation)
	}
	if current.FailureReason != "transient decoder failure" {
		t.Errorf("retry discarded original failure: %q", current.FailureReason)
	}
	if t.Failed() {
		return
	}
	if _, err := s.Commit(ctx, application.CommitRequest{RequestID: second.RequestID, CredentialID: second.CredentialID, Generation: second.Generation, ExpectedVersion: second.Version, Payload: map[string]any{"ok": true}}); err != nil {
		t.Fatal(err)
	}
}
