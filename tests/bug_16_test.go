package tests

import (
	"context"
	"errors"
	"hwj-macgo-0018/application"
	"hwj-macgo-0018/repository"
	"sync"
	"testing"
)

func TestBug16ConcurrentCommitOptimisticVersion(t *testing.T) {
	store := repository.NewMemoryStore()
	ctx := context.Background()
	initial := repository.Item{Kind: "request", ID: "shared", Version: 1, Data: map[string]any{"status": "occupied"}}
	if _, err := store.Put(ctx, initial); err != nil {
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
	first.Version++
	second.Version++
	first.Data["worker"] = "one"
	second.Data["worker"] = "two"
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, pair := range []struct {
		tx   repository.Transaction
		item repository.Item
	}{{tx1, first}, {tx2, second}} {
		wg.Add(1)
		go func(tx repository.Transaction, item repository.Item) {
			defer wg.Done()
			<-start
			results <- application.CommitRequestVersion(ctx, tx, item)
		}(pair.tx, pair.item)
	}
	close(start)
	wg.Wait()
	close(results)
	var success, conflicts int
	for commitErr := range results {
		switch {
		case commitErr == nil:
			success++
		case errors.Is(commitErr, repository.ErrOptimisticLock):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent commit error: %v", commitErr)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("expected one committed version and one conflict, got success=%d conflicts=%d", success, conflicts)
	}
	got, err := store.Get(ctx, "request", "shared")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.Data["worker"] == nil {
		t.Fatalf("concurrent update changed state incorrectly: %#v", got)
	}
}
