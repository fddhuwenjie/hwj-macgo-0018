package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hwj-macgo-0018/recovery"
)

type bug22Clock struct{ now time.Time }

func (c *bug22Clock) Now() time.Time { return c.now }

type bug22Provider struct {
	data []byte
	err  error
}

func (p *bug22Provider) Snapshot(context.Context) ([]byte, error) {
	return append([]byte(nil), p.data...), p.err
}

func TestBug22FailedSnapshotLeavesNoCandidateDiagnosis(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	clock := &bug22Clock{now: time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)}
	provider := &bug22Provider{data: []byte("complete-v1")}
	service := recovery.NewService(dir, clock, provider)
	if err := service.Rotate(ctx, 3); err != nil {
		t.Fatalf("write initial snapshot: %v", err)
	}

	clock.now = clock.now.Add(time.Minute)
	provider.data = []byte("half-v2")
	provider.err = errors.New("snapshot source interrupted")
	if err := service.Rotate(ctx, 3); err == nil {
		t.Fatal("failed snapshot unexpectedly succeeded")
	}
	entries, err := os.ReadDir(service.SnapshotDir())
	if err != nil {
		t.Fatalf("read snapshot directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("failed snapshot left candidate %q", entry.Name())
		}
	}

	record, err := recovery.EncodeLogRecord(recovery.LogRecord{
		Sequence: 2, Version: 2, Data: []byte("continued-after-v1"),
	})
	if err != nil {
		t.Fatalf("encode continuation log: %v", err)
	}
	logPath := filepath.Join(service.LogDir(), "journal-0001.log")
	if err := os.WriteFile(logPath, record, 0o644); err != nil {
		t.Fatalf("write continuation log: %v", err)
	}

	reopened := recovery.NewService(dir, clock, provider)
	result, err := reopened.Replay(ctx)
	if err != nil {
		t.Fatalf("replay after failed snapshot: %v", err)
	}
	if result.SnapshotVersion != 1 || result.LastSequence != 2 || result.Applied != 1 || result.Truncated {
		t.Errorf("recovery used wrong checkpoint: snapshot=%d last=%d applied=%d truncated=%v",
			result.SnapshotVersion, result.LastSequence, result.Applied, result.Truncated)
	}
	if info, err := os.Stat(logPath); err != nil || info.Size() != int64(len(record)) {
		t.Errorf("valid continuation log changed after recovery: size=%v err=%v", func() int64 {
			if info == nil {
				return -1
			}
			return info.Size()
		}(), err)
	}

	provider.err = nil
	provider.data = []byte("complete-v2")
	clock.now = clock.now.Add(time.Minute)
	if err := reopened.Rotate(ctx, 3); err != nil {
		t.Fatalf("legal rotation after failure: %v", err)
	}
	latest, err := reopened.SnapshotLatest(ctx)
	if err != nil {
		t.Fatalf("load snapshot after legal rotation: %v", err)
	}
	if latest.Version != 2 || string(latest.Data) != "complete-v2" {
		t.Fatalf("legal rotation continued from failed version: version=%d data=%q", latest.Version, latest.Data)
	}
}
