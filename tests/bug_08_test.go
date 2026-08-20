package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/repository"
)

func TestBug08ReopenRebuildsIdempotencyIndex(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := repository.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := application.NewService(store, func() time.Time { return time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC) })
	one, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "caller", NamespaceID: "ns-a", Key: "same", Digest: "a"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "caller", NamespaceID: "ns-b", Key: "same", Digest: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := repository.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s2 := application.NewService(reopened)
	got, err := s2.PreRegister(ctx, application.PreRegisterRequest{CallerID: "caller", NamespaceID: "ns-a", Key: "same", Digest: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestID != one.RequestID || got.Created {
		t.Errorf("reopen lost ns-a idempotency: %#v want %s", got, one.RequestID)
	}

	// The other namespace must stay independently recoverable: it was
	// persisted alongside ns-a and must not collapse into ns-a's request
	// when the index is rebuilt. Reusing the original namespace+digest
	// pair should hit the ns-b request, not create a new one.
	gotB, err := s2.PreRegister(ctx, application.PreRegisterRequest{CallerID: "caller", NamespaceID: "ns-b", Key: "same", Digest: "b"})
	if err != nil {
		t.Errorf("reopen rejected independent ns-b request: %v", err)
	}
	if err == nil && (gotB.Created || gotB.RequestID != two.RequestID) {
		t.Errorf("ns-b request was not recovered across reopen: got=%#v want=%s", gotB, two.RequestID)
	}
	if err == nil && gotB.RequestID == got.RequestID {
		t.Errorf("ns-a and ns-b collapsed into one request after reopen")
	}
}
