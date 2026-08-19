package tests

import (
	"testing"
	"time"

	"hwj-macgo-0018/query"
)

func TestBug30ReplayHistoryStableAfterClockRollback(t *testing.T) {
	base := time.Date(2026, time.January, 3, 10, 0, 0, 0, time.UTC)
	rows := []query.Row{
		{ID: "generation-1", Fields: map[string]any{"generation": int64(1), "hit_at": base}},
		{ID: "generation-2", Fields: map[string]any{"generation": int64(2), "hit_at": base.Add(-time.Hour)}},
	}
	ordered := query.NewService(rows).SortMulti([]query.SortKey{{Key: "hit_at"}}).All()
	if ordered[0].Fields["generation"] != int64(1) || ordered[1].Fields["generation"] != int64(2) {
		t.Fatalf("clock rollback reversed replay generations: %#v", ordered)
	}
}
