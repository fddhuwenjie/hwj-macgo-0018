package tests

import (
	"context"
	"os"
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
	snapshots := []recovery.Snapshot{
		{Version: 5, CreatedAt: base.Add(3 * time.Hour), Data: []byte("v5-late-clock")},
		{Version: 6, CreatedAt: base, Data: []byte("v6-first")},
		{Version: 6, CreatedAt: base.Add(time.Hour), Data: []byte("v6-stable-winner")},
	}
	for _, snapshot := range snapshots {
		if err := store.Save(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshots", "snap-corrupt.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := recovery.NewService(dir, fixedClock{now: base.Add(4 * time.Hour)}, nil)
	latest, err := service.SnapshotLatest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != 6 || string(latest.Data) != "v6-stable-winner" {
		t.Errorf("expected highest logical version and stable same-version winner, got %#v", latest)
	}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 3 {
		t.Errorf("corrupt candidate was not isolated: %#v", listed)
	} else if listed[0].Version != 5 || listed[1].Version != 6 || listed[2].Version != 6 || !listed[1].CreatedAt.Before(listed[2].CreatedAt) {
		t.Errorf("snapshot list is not ordered by version then creation time: %#v", listed)
	}

	frame, err := recovery.EncodeLogRecord(recovery.LogRecord{Sequence: 7, Version: 7, Data: []byte("after-v6")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(service.LogDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.LogDir(), "000007.log"), frame, 0o644); err != nil {
		t.Fatal(err)
	}

	reopened := recovery.NewService(dir, fixedClock{now: base.Add(5 * time.Hour)}, nil)
	result, err := reopened.Replay(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.SnapshotVersion != 6 || result.LastSequence != 7 || result.Applied != 1 || result.Truncated {
		t.Errorf("reopen did not continue the log after logical version 6: %#v", result)
	}
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }
