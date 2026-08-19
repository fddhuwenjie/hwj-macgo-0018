package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"hwj-macgo-0018/application"
)

func TestBug07ConcurrentTakeoverSingleGeneration(t *testing.T) {
	clock := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
	s := application.NewService(func() time.Time { return clock })
	ctx := context.Background()
	pr, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "parallel", NamespaceID: "lease", Key: "shared-07", Digest: "d07"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * time.Minute)
	results := make(chan application.OccupyResponse, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, e := s.Occupy(ctx, application.OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: first.Version, LeaseTTL: time.Minute}); e == nil {
				results <- got
			}
		}()
	}
	wg.Wait()
	close(results)
	seen := map[string]bool{}
	for got := range results {
		seen[got.CredentialID] = true
	}
	if len(seen) > 1 {
		t.Fatalf("concurrent takeover created multiple active credentials: %v", seen)
	}
	if len(seen) == 0 {
		t.Fatal("all takeover attempts failed")
	}
}
