package tests

import (
	"context"
	"errors"
	"testing"

	"hwj-macgo-0018/application"
)

func TestBug02NamespaceScopedDigest(t *testing.T) {
	s := application.NewService()
	ctx := context.Background()
	a, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "robot", NamespaceID: "north", Key: "frame-1", Digest: "digest-a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "robot", NamespaceID: "south", Key: "frame-1", Digest: "digest-b"})
	if err != nil {
		t.Fatalf("different namespace must not conflict: %v", err)
	}
	if b.RequestID == a.RequestID || !b.Created {
		t.Fatalf("namespace collision merged requests: a=%#v b=%#v", a, b)
	}
	_, err = s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "robot", NamespaceID: "north", Key: "frame-1", Digest: "digest-conflict"})
	if !errors.Is(err, application.ErrConflict) {
		t.Fatalf("same namespace accepted a conflicting digest: %v", err)
	}
	replayed, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "robot", NamespaceID: "south", Key: "frame-1", Digest: "digest-b"})
	if err != nil || replayed.Created || replayed.RequestID != b.RequestID {
		t.Fatalf("legal idempotent retry failed after scoped conflict: response=%#v err=%v", replayed, err)
	}
	page, err := s.QueryRequests(ctx, application.QueryRequest{CallerID: "robot"})
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("scoped conflict changed existing requests: page=%#v err=%v", page, err)
	}
	batch := application.NewCommandBatchResult(0)
	batch.AddFailed("south/frame-1", errors.New("digest conflict"))
	if batch.Failed != 1 || batch.Succeeded != 0 {
		t.Fatalf("partial result counters are inconsistent: %#v", batch)
	}
}
