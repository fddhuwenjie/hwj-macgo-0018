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
}
