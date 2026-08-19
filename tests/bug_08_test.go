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
	if err != nil { t.Fatal(err) }
	s := application.NewService(store, func() time.Time { return time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC) })
	one, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID:"caller", NamespaceID:"ns-a", Key:"same", Digest:"a"})
	if err != nil { t.Fatal(err) }
	if _, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID:"caller", NamespaceID:"ns-b", Key:"same", Digest:"b"}); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }
	reopened, err := repository.NewFileStore(dir)
	if err != nil { t.Fatal(err) }
	s2 := application.NewService(reopened)
	got, err := s2.PreRegister(ctx, application.PreRegisterRequest{CallerID:"caller", NamespaceID:"ns-a", Key:"same", Digest:"a"})
	if err != nil { t.Fatal(err) }
	if got.RequestID != one.RequestID || got.Created { t.Fatalf("reopen lost scoped idempotency: %#v want %s", got, one.RequestID) }
}
