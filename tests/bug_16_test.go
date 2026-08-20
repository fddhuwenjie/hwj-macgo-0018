package tests

import (
	"context"
	"errors"
	"sync"
	"testing"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/repository"
)

type bug16CommitOutcome struct {
	worker string
	err    error
}

func TestBug16ConcurrentCommitOptimisticVersion(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	initial, err := store.Put(ctx, repository.Item{
		Kind: "request", ID: "shared", Version: 1,
		Data: map[string]any{"status": "occupied", "worker": "none"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tx1, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx2, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := tx1.Get("request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	second, err := tx2.Get("request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	first.Version = initial.Version + 1
	second.Version = initial.Version + 1
	first.Data["worker"] = "one"
	second.Data["worker"] = "two"

	start := make(chan struct{})
	outcomes := make(chan bug16CommitOutcome, 2)
	var wg sync.WaitGroup
	for _, candidate := range []struct {
		worker string
		tx     repository.Transaction
		item   repository.Item
	}{{worker: "one", tx: tx1, item: first}, {worker: "two", tx: tx2, item: second}} {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outcomes <- bug16CommitOutcome{
				worker: candidate.worker,
				err:    application.CommitRequestVersion(ctx, candidate.tx, candidate.item),
			}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)

	successes := 0
	conflicts := 0
	winner := ""
	for outcome := range outcomes {
		switch {
		case outcome.err == nil:
			successes++
			winner = outcome.worker
		case errors.Is(outcome.err, repository.ErrOptimisticLock):
			conflicts++
		default:
			t.Errorf("worker %s returned unexpected error: %v", outcome.worker, outcome.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("expected one winner and one optimistic conflict, got successes=%d conflicts=%d", successes, conflicts)
	}

	committed, err := store.Get(ctx, "request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	if committed.Version != initial.Version+1 {
		t.Errorf("concurrent commits produced version %d, want %d", committed.Version, initial.Version+1)
	}
	if winner != "" && committed.Data["worker"] != winner {
		t.Errorf("stored payload belongs to %v, but successful worker was %s", committed.Data["worker"], winner)
	}

	// A conflict must not poison the record: a later transaction can advance it once.
	followUp, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next, err := followUp.Get("request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	next.Version = committed.Version + 1
	next.Data["worker"] = "three"
	if err := application.CommitRequestVersion(ctx, followUp, next); err != nil {
		t.Fatal(err)
	}
	afterFollowUp, err := store.Get(ctx, "request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	if afterFollowUp.Version != committed.Version+1 || afterFollowUp.Data["worker"] != "three" {
		t.Errorf("legal follow-up did not advance exactly once: %#v", afterFollowUp)
	}
}
