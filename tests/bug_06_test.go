package tests

import (
	"context"
	"testing"

	"hwj-macgo-0018/application"
)

func TestBug06BatchPartialFailure(t *testing.T) {
	s := application.NewService()
	ctx := context.Background()
	items := []application.PreRegisterRequest{
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "one", Digest: "d1"},
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "one", Digest: "different"},
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "three", Digest: "d3"},
	}
	results := s.PreRegisterBatch(ctx, items)
	if !results[0].Success || results[1].Success || !results[2].Success {
		t.Errorf("batch did not preserve per-item outcomes: %#v", results)
	}
	third, err := s.PreRegister(ctx, items[2])
	if err != nil || third.Created || third.RequestID != results[2].ID {
		t.Errorf("third item was not persisted by the original batch: response=%#v err=%v", third, err)
	}
	r := application.NewCommandBatchResult(0)
	r.AddFailed("one", context.Canceled)
	if r.Total != 1 || r.Failed != 1 {
		t.Errorf("failed count drifted: %#v", r)
	}
}

// TestBug06BatchIdempotentRetry locks in the idempotent retry boundary: after
// a batch that partially fails on a middle digest conflict, retrying the whole
// batch must return the already-succeeded items idempotently (same id, not
// re-created), keep reporting the conflict, and never double-write. The failure
// summary must also match the actual number of entries.
func TestBug06BatchIdempotentRetry(t *testing.T) {
	s := application.NewService()
	ctx := context.Background()
	items := []application.PreRegisterRequest{
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "one", Digest: "d1"},
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "one", Digest: "different"},
		{CallerID: "tenant-a", NamespaceID: "scope", Key: "three", Digest: "d3"},
	}

	// First batch: one succeeds, the middle item conflicts, three succeeds.
	first := s.PreRegisterBatch(ctx, items)
	if !first[0].Success || first[1].Success || !first[2].Success {
		t.Errorf("first batch did not preserve per-item outcomes: %#v", first)
	}
	oneID, threeID := first[0].ID, first[2].ID

	// Client retries the whole batch. Already-succeeded items must come back
	// idempotently (same id, not re-created) and the conflict must still be
	// reported, so the caller can tell what is already durable.
	second := s.PreRegisterBatch(ctx, items)
	if !second[0].Success || second[1].Success || !second[2].Success {
		t.Errorf("retry batch did not preserve per-item outcomes: %#v", second)
	}
	if second[0].ID != oneID || second[2].ID != threeID {
		t.Errorf("retry did not return existing ids: got %q/%q want %q/%q", second[0].ID, second[2].ID, oneID, threeID)
	}

	// PreRegister itself must report the stored item as not created on retry.
	resp, err := s.PreRegister(ctx, items[0])
	if err != nil || resp.Created {
		t.Errorf("retry re-created item instead of returning it: %#v err=%v", resp, err)
	}
	third, err := s.PreRegister(ctx, items[2])
	if err != nil || third.Created || third.RequestID != threeID {
		t.Errorf("later successful item was not durable across retry: %#v err=%v", third, err)
	}

	// Failure summary counts must match the actual number of entries.
	summary := application.NewCommandBatchResult(3)
	summary.AddSucceeded(items[0].Key)
	summary.AddFailed(items[1].Key, application.ErrConflict)
	summary.AddSucceeded(items[2].Key)
	if summary.Total != 3 || summary.Succeeded != 2 || summary.Failed != 1 {
		t.Errorf("summary counts drifted: %#v", summary)
	}
	if !summary.HasPartialFailure() {
		t.Errorf("expected partial failure to be reported")
	}
}
