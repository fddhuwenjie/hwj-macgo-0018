package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/audit"
	"hwj-macgo-0018/query"
)

type bug30Persister struct {
	mu   sync.Mutex
	data map[string]any
}

func (p *bug30Persister) Load() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneBug30Data(p.data), nil
}

func (p *bug30Persister) Save(data map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = cloneBug30Data(data)
	return nil
}

func cloneBug30Data(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

type bug30Replay struct {
	Generation int64
	HitAt      time.Time
}

func TestBug30ReplayHistoryStableAfterClockRollback(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, time.January, 3, 10, 0, 0, 0, time.UTC)
	now := base
	store := &bug30Persister{}
	service := application.NewService(store, func() time.Time { return now })

	pre, err := service.PreRegister(ctx, application.PreRegisterRequest{CallerID: "history", NamespaceID: "replay", Key: "clock-rollback", Digest: "digest", LeaseTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := service.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occupied.CredentialID, Generation: occupied.Generation, ExpectedVersion: occupied.Version, Payload: map[string]any{"generation": 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID}); err != nil {
		t.Fatal(err)
	}
	if err := service.PrepareGenerationForReplay(pre.RequestID); err != nil {
		t.Fatal(err)
	}
	current, err := service.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	now = base.Add(-time.Hour)
	if _, err := service.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occupied.CredentialID, Generation: current.Generation, ExpectedVersion: current.Version, Payload: map[string]any{"generation": 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID}); err != nil {
		t.Fatal(err)
	}

	reopened := application.NewService(store, func() time.Time { return now })
	stats, err := reopened.ReplayHitRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Committed != 1 || stats.Replays != 2 {
		t.Fatalf("reopen did not restore both replay events: %#v", stats)
	}

	persisted, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(persisted["replays"])
	if err != nil {
		t.Fatal(err)
	}
	history := map[string][]bug30Replay{}
	if err := json.Unmarshal(raw, &history); err != nil {
		t.Fatal(err)
	}
	replays := history[pre.RequestID]
	if len(replays) != 2 {
		t.Fatalf("persisted replay history=%#v", history)
	}
	rows := make([]query.Row, 0, len(replays))
	for i, replay := range replays {
		rows = append(rows, query.Row{ID: strconv.Itoa(i + 1), Fields: map[string]any{"generation": replay.Generation, "hit_at": replay.HitAt}})
	}
	ordered := query.NewService(rows).SortMulti([]query.SortKey{{Key: "hit_at"}}).All()
	if ordered[0].Fields["generation"] != int64(1) || ordered[1].Fields["generation"] != int64(2) {
		t.Errorf("clock rollback reversed persisted replay generations: %#v", ordered)
	}

	auditPath := t.TempDir() + "/replay-history.audit"
	chain, err := audit.Open(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range ordered {
		generation := strconv.FormatInt(row.Fields["generation"].(int64), 10)
		if _, err := chain.Append(ctx, "history-export", "replay", "request", pre.RequestID, map[string]string{"generation": generation}); err != nil {
			t.Fatal(err)
		}
	}
	if err := chain.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedChain, err := audit.Open(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedChain.Close()

	var exported bytes.Buffer
	if err := reopenedChain.Export(&exported); err != nil {
		t.Fatal(err)
	}
	events, err := audit.VerifyExport(&exported)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Details["generation"] != "1" || events[1].Details["generation"] != "2" {
		t.Errorf("verified export preserved the wrong replay order: %#v", events)
	}
}
