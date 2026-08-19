package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hwj-macgo-0018/recovery"
)

func TestBug27RecoveryChoosesHighestSnapshotVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := recovery.NewFileSnapshotStore(filepath.Join(dir, "snapshots"))
	base := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	if err := store.Save(ctx, recovery.Snapshot{Version: 5, CreatedAt: base.Add(time.Hour), Data: []byte("v5")}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, recovery.Snapshot{Version: 6, CreatedAt: base, Data: []byte("v6")}); err != nil {
		t.Fatal(err)
	}
	service := recovery.NewService(dir, fixedClock{now: base.Add(2 * time.Hour)}, nil)
	latest, err := service.SnapshotLatest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != 6 {
		t.Fatalf("expected highest logical snapshot version 6, got %d", latest.Version)
	}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[len(listed)-1].Version != 6 {
		t.Fatalf("expected snapshot list ordered by logical version, got %#v", listed)
	}
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }
