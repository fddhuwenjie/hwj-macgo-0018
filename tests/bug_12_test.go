package tests

import (
	"hwj-macgo-0018/query"
	"testing"
)

func TestBug12FilterSortThenPaginate(t *testing.T) {
	rows := []query.Row{{ID: "low", Fields: map[string]any{"amount": 5}}, {ID: "mid", Fields: map[string]any{"amount": 20}}, {ID: "high", Fields: map[string]any{"amount": 30}}}
	if got := query.NewService(rows).Filter(query.HasField("amount")).Len(); got != 3 {
		t.Fatalf("field filter removed valid rows: %d", got)
	}
	s := query.NewService(rows)
	s.Filter(func(r query.Row) bool { v, _ := r.Fields["amount"].(int); return v >= 20 }).Sort("amount", false)
	p := s.Page(1, 1)
	if len(p.Items) != 1 || p.Items[0].ID != "high" {
		t.Fatalf("filtered page=%#v", p.Items)
	}
}
