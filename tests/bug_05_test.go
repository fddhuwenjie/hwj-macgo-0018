package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/query"
)

func TestBug05ExpirationScanDiagnosis(t *testing.T) {
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	clock := func() time.Time { return now }
	s := application.NewService(clock)
	ctx := context.Background()
	pr, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "scanner", NamespaceID: "leases", Key: "item-5", Digest: "d5", LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: time.Minute}); err != nil {
		t.Fatal(err)
	}
	batch, err := s.ExpireCredentials(ctx, now.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Expired) != 0 {
		t.Fatalf("diagnosis expected the injected scan to skip the expired credential: %#v", batch)
	}
	rows := query.NewService([]query.Row{{ID: "expired", Fields: map[string]any{"status": "executing", "expires_at": now.Add(-time.Second)}}})
	if got := rows.HangingExecutions(now); len(got) != 0 {
		t.Fatalf("diagnosis expected the injected query to omit the expired row: %#v", got)
	}
}
