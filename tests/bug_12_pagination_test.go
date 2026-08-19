package tests

import (
	"hwj-macgo-0018/query"
	"testing"
)

// bug12Rows builds a mixed data set: most rows carry an "amount" field used for
// filtering and sorting, while one row omits it so HasField is exercised.
func bug12Rows() []query.Row {
	return []query.Row{
		{ID: "a", Fields: map[string]any{"amount": 5}},
		{ID: "b", Fields: map[string]any{"amount": 20}},
		{ID: "c", Fields: map[string]any{"amount": 30}},
		{ID: "d", Fields: map[string]any{"amount": 10}},
		{ID: "e", Fields: map[string]any{"note": "no amount"}},
		{ID: "f", Fields: map[string]any{"amount": 25}},
	}
}

// TestBug12HasFieldKeepsFieldRows guards the inverted HasField regression: rows
// that carry the field must survive the filter, rows without it must not.
func TestBug12HasFieldKeepsFieldRows(t *testing.T) {
	s := query.NewService(bug12Rows()).Filter(query.HasField("amount"))
	if got := s.Len(); got != 5 {
		t.Fatalf("HasField should keep the 5 rows that carry amount, got %d", got)
	}
	for _, row := range s.All() {
		if _, ok := row.Fields["amount"]; !ok {
			t.Fatalf("HasField leaked a row without the amount field: %s", row.ID)
		}
	}
}

// TestBug12PaginationCoversAllRows pages through a filtered+sorted list and
// asserts every page boundary is correct: no duplicate, no missing item, and
// reading past the end yields an empty page instead of a panic.
func TestBug12PaginationCoversAllRows(t *testing.T) {
	const limit = 2
	// Expected order: HasField drops "e", ascending amount gives a(5),d(10),b(20),f(25),c(30).
	want := []string{"a", "d", "b", "f", "c"}

	s := query.NewService(bug12Rows()).
		Filter(query.HasField("amount")).
		Sort("amount", false)
	total := s.Len()

	var got []string
	for offset := 0; offset < total; offset += limit {
		page := s.Page(offset, limit)
		if page.Total != total {
			t.Fatalf("offset=%d: Total=%d want %d", offset, page.Total, total)
		}
		if page.Offset != offset {
			t.Fatalf("offset=%d: echoed Offset=%d", offset, page.Offset)
		}
		if len(page.Items) > limit {
			t.Fatalf("offset=%d: page returned %d items, limit is %d", offset, len(page.Items), limit)
		}
		if len(page.Items) == 0 {
			t.Fatalf("offset=%d: empty page before walking all %d rows", offset, total)
		}
		for _, row := range page.Items {
			got = append(got, row.ID)
		}
	}

	// One page beyond the last must yield an empty page, not a panic.
	if tail := s.Page(total, limit); len(tail.Items) != 0 {
		t.Fatalf("offset==total should be empty, got %#v", tail.Items)
	}

	if len(got) != len(want) {
		t.Fatalf("page walk collected %d ids, want %d (missing/extra items): %v", len(got), len(want), got)
	}
	seen := make(map[string]int, len(got))
	for i, id := range got {
		if seen[id]++; seen[id] > 1 {
			t.Fatalf("row %s appeared more than once across pages: %v", id, got)
		}
		if id != want[i] {
			t.Fatalf("page %d produced wrong record %s, want %s (full walk: %v)", i/limit, id, want[i], got)
		}
	}
}

// TestBug12PaginationBeyondEndNoPanic checks the previously crashing boundary
// where offset exceeded the absolute limit index; it must now return empty.
func TestBug12PaginationBeyondEndNoPanic(t *testing.T) {
	s := query.NewService(bug12Rows()).Sort("amount", false)

	// offset past total must clamp and return an empty page, never panic.
	p := s.Page(100, 1)
	if len(p.Items) != 0 {
		t.Fatalf("offset beyond total should yield empty page, got %#v", p.Items)
	}

	// offset inside the set with a limit that would have made end<offset under
	// the old absolute-index logic: must return exactly the remaining row.
	p = s.Page(5, 1)
	if len(p.Items) != 1 {
		t.Fatalf("last single-item page expected, got %#v", p.Items)
	}
}
