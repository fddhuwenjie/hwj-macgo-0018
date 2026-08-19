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
		t.Fatalf("batch did not preserve per-item outcomes: %#v", results)
	}
	if _, err := s.PreRegister(ctx, items[2]); err != nil {
		t.Fatalf("third item was not persisted: %v", err)
	}
	r := application.NewCommandBatchResult(0)
	r.AddFailed("one", context.Canceled)
	if r.Total != 1 || r.Failed != 1 {
		t.Fatalf("failed count drifted: %#v", r)
	}
}
