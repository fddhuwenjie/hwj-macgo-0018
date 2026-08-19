package tests

import (
	"testing"

	"hwj-macgo-0018/query"
)

func TestBug24StableWaitPaginationDiagnosis(t *testing.T) {
	rows := []query.Row{
		{ID: "a", Fields: map[string]any{"wait_ms": 100, "version": 1}},
		{ID: "b", Fields: map[string]any{"wait_ms": 100, "version": 1}},
		{ID: "c", Fields: map[string]any{"wait_ms": 100, "version": 1}},
		{ID: "d", Fields: map[string]any{"wait_ms": 100, "version": 1}},
		{ID: "e", Fields: map[string]any{"wait_ms": 100, "version": 1}},
	}
	first := query.NewService(rows).Sort("wait_ms", false).Page(0, 3)
	rows[4].Fields["version"] = 2
	second := query.NewService(rows).Sort("wait_ms", false).Page(3, 3)

	seen := map[string]bool{}
	for _, row := range first.Items {
		seen[row.ID] = true
	}
	duplicate := ""
	for _, row := range second.Items {
		if seen[row.ID] {
			duplicate = row.ID
		}
	}
	if duplicate != "" {
		t.Fatalf("version-only update moved the page boundary and repeated %q", duplicate)
	}
	if len(first.Items)+len(second.Items) != 5 {
		t.Fatalf("expected five unique rows across pages, got %d", len(first.Items)+len(second.Items))
	}
}
