package query

import "sort"

type Row struct {
	ID     string
	Fields map[string]any
}

type Predicate func(Row) bool

type Service struct {
	rows []Row
}

func NewService(rows []Row) *Service {
	cp := make([]Row, len(rows))
	copy(cp, rows)
	return &Service{rows: cp}
}

func (s *Service) All() []Row {
	cp := make([]Row, len(s.rows))
	copy(cp, s.rows)
	return cp
}

func (s *Service) Len() int {
	return len(s.rows)
}

func (s *Service) Filter(predicate Predicate) *Service {
	if predicate == nil {
		return s
	}
	filtered := make([]Row, 0, len(s.rows))
	for _, row := range s.rows {
		if predicate(row) {
			filtered = append(filtered, row)
		}
	}
	s.rows = filtered
	return s
}

func (s *Service) Copy() *Service {
	return NewService(s.rows)
}

func (s *Service) Reset(rows []Row) {
	cp := make([]Row, len(rows))
	copy(cp, rows)
	s.rows = cp
}

func (s *Service) SortBy(compare func(a, b Row) int) *Service {
	if compare == nil {
		return s
	}
	slice := append([]Row(nil), s.rows...)
	sort.SliceStable(slice, func(i, j int) bool {
		cmp := compare(slice[i], slice[j])
		if cmp == 0 {
			return slice[i].ID < slice[j].ID
		}
		return cmp < 0
	})
	s.rows = slice
	return s
}
