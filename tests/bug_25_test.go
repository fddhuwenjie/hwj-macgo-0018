package tests

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/query"
)

type bug25Persister struct {
	mu   sync.Mutex
	data map[string]any
}

func (p *bug25Persister) Load() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneBug25Data(p.data), nil
}

func (p *bug25Persister) Save(data map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = cloneBug25Data(data)
	return nil
}

func cloneBug25Data(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func commitForRate(t *testing.T, app *application.UseCase, key string) application.CommitResponse {
	t.Helper()
	ctx := context.Background()
	pre, err := app.PreRegister(ctx, application.PreRegisterRequest{CallerID: "rate", NamespaceID: "replay", Key: key, Digest: "digest-" + key, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := app.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := app.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occupied.CredentialID, Generation: occupied.Generation, ExpectedVersion: occupied.Version, Payload: map[string]any{"key": key}})
	if err != nil {
		t.Fatal(err)
	}
	return committed
}

func TestBug25ReplayHitRateDenominator(t *testing.T) {
	ctx := context.Background()
	store := &bug25Persister{}
	app := application.NewUseCase(store)
	first := commitForRate(t, app, "first")
	second := commitForRate(t, app, "second")
	for i := 0; i < 3; i++ {
		if _, err := app.Replay(ctx, application.ReplayRequest{RequestID: first.RequestID}); err != nil {
			t.Fatal(err)
		}
	}
	rate, err := app.ReplayHitRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(rate.Rate-0.5) > 1e-9 {
		t.Errorf("expected replay hit rate 0.5 for one of two committed requests, got %.6f", rate.Rate)
	}
	if rate.Committed != 2 || rate.Replays != 3 {
		t.Errorf("statistics lost committed or replay records: %#v", rate)
	}

	derived := query.NewService([]query.Row{
		{ID: "first", Fields: map[string]any{"status": "committed", "replay_hit": true}},
		{ID: "second", Fields: map[string]any{"status": "committed", "replay_hit": false}},
	}).ReplayHitRate()
	if math.Abs(derived-0.5) > 1e-9 {
		t.Errorf("expected derived replay hit rate 0.5, got %.6f", derived)
	}

	reopened := application.NewUseCase(store)
	afterReopen, err := reopened.ReplayHitRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if afterReopen.Committed != 2 || afterReopen.Replays != 3 || math.Abs(afterReopen.Rate-0.5) > 1e-9 {
		t.Errorf("reopen changed request-based replay statistics: %#v", afterReopen)
	}
	if _, err := reopened.Replay(ctx, application.ReplayRequest{RequestID: second.RequestID}); err != nil {
		t.Fatal(err)
	}
	allHit, err := reopened.ReplayHitRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if allHit.Committed != 2 || allHit.Replays != 4 || math.Abs(allHit.Rate-1) > 1e-9 {
		t.Errorf("a legitimate replay after reopen did not produce two-of-two hit rate: %#v", allHit)
	}
	derivedAllHit := query.NewService([]query.Row{
		{ID: first.RequestID, Fields: map[string]any{"status": "committed", "replay_hit": true}},
		{ID: second.RequestID, Fields: map[string]any{"status": "committed", "replay_hit": true}},
	}).ReplayHitRate()
	if math.Abs(derivedAllHit-1) > 1e-9 {
		t.Errorf("derived query did not report both committed requests as hits: %.6f", derivedAllHit)
	}
}
