package tests

import (
	"hwj-macgo-0018/query"
	"testing"
)

func TestBug26FailureAggregationByGenerationDiagnosis(t *testing.T) {
	rows := []query.Row{
		{ID: "f1", Fields: map[string]any{"caller": "worker", "namespace": "render", "key": "scene", "generation": uint64(1), "status": "failed", "reason": "timeout"}},
		{ID: "f2", Fields: map[string]any{"caller": "worker", "namespace": "render", "key": "scene", "generation": uint64(2), "status": "failed", "reason": "lease lost"}},
	}
	groups := query.NewService(rows).ConflictRequests()
	if len(groups) != 2 {
		t.Fatalf("failure history merged generations into %d aggregate(s)", len(groups))
	}
	// Each generation's failure must survive as its own aggregate carrying its
	// own reason, so a query can distinguish the timeout (gen 1) from the lease
	// loss (gen 2) even after the store has been reopened.
	byGen := make(map[uint64]query.ConflictRequest, len(groups))
	for _, g := range groups {
		byGen[g.Generation] = g
	}
	gen1, ok := byGen[1]
	if !ok {
		t.Fatalf("generation 1 failure was not preserved as its own aggregate")
	}
	if len(gen1.Reasons) != 1 || gen1.Reasons[0] != "timeout" {
		t.Fatalf("generation 1 reasons = %v, want [timeout]", gen1.Reasons)
	}
	gen2, ok := byGen[2]
	if !ok {
		t.Fatalf("generation 2 failure was not preserved as its own aggregate")
	}
	if len(gen2.Reasons) != 1 || gen2.Reasons[0] != "lease lost" {
		t.Fatalf("generation 2 reasons = %v, want [lease lost]", gen2.Reasons)
	}
}
