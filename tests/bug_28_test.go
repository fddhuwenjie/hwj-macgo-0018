package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/audit"
	"hwj-macgo-0018/query"
)

func TestBug28AuditExportDeepCopy(t *testing.T) {
	chain, err := audit.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chain.Append(context.Background(), "worker", "write", "job", "j1", map[string]string{"state": "ready"}); err != nil {
		t.Fatal(err)
	}
	events := chain.Events()
	events[0].Details["state"] = "tampered"
	if err := chain.Verify(); err != nil {
		t.Fatalf("audit chain changed through returned details: %v", err)
	}
	rows := query.NewService([]query.Row{{ID: "j1", Fields: map[string]any{"state": "ready", "at": time.Now()}}})
	returned := rows.All()
	returned[0].Fields["state"] = "tampered"
	if rows.All()[0].Fields["state"] != "ready" {
		t.Fatal("query All exposed shared field map")
	}
}
