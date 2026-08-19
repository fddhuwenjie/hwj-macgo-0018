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
	batch := application.NewCommandBatchResult(0)
	batch.AddFailed("south/frame-1", errors.New("digest conflict"))
	if batch.Failed != 1 || batch.Succeeded != 0 {
		t.Fatalf("partial result counters are inconsistent: %#v", batch)
	}
}
