package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
)

func TestBug29BatchOccupyContinuesAfterFailure(t *testing.T) {
	ctx := context.Background()
	app := application.NewUseCase()
	items := make([]application.OccupyRequest, 3)
	for i := range items {
		pre, err := app.PreRegister(ctx, application.PreRegisterRequest{CallerID: "batch", NamespaceID: "lease", Key: string(rune('a' + i)), Digest: "d", LeaseTTL: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		items[i] = application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: time.Minute}
	}
	items[1].ExpectedVersion++
	results := app.OccupyBatch(ctx, items)
	if len(results) != 3 {
		t.Fatalf("expected all batch items to be reported, got %d", len(results))
	}
	if results[0].ID == "" || results[1].Error == "" || results[2].ID != "" || results[2].Error != "" {
		t.Fatalf("diagnosis expected the third item to remain unprocessed after the stale second item: %#v", results)
	}
}
