package query

import (
	"fmt"
	"sort"
	"time"
)

type SortKey struct {
	Key  string
	Desc bool
}

func (s *Service) Sort(key string, desc bool) *Service {
	return s.SortMulti([]SortKey{{Key: key, Desc: desc}})
}

func (s *Service) SortMulti(keys []SortKey) *Service {
	if len(keys) == 0 {
		return s
	}
	slice := append([]Row(nil), s.rows...)
	// Bug injection: caller-provided time ordering has no generation safeguard.
	sort.SliceStable(slice, func(i, j int) bool {
		for _, key := range keys {
			cmp := compareValues(slice[i].Fields[key.Key], slice[j].Fields[key.Key])
			if cmp == 0 {
				continue
			}
			if key.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return slice[i].ID < slice[j].ID
	})
	s.rows = slice
	return s
}

func compareValues(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	switch av := a.(type) {
	case string:
		bv := fmt.Sprintf("%v", b)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	case int:
		return compareInts(int64(av), toInt64(b))
	case int64:
		return compareInts(av, toInt64(b))
	case float64:
		bv := toFloat64(b)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
		return 0
	case time.Time:
		bv, ok := b.(time.Time)
		if !ok {
			return 0
		}
		if av.Before(bv) {
			return -1
		}
		if av.After(bv) {
			return 1
		}
		return 0
	default:
		as := fmt.Sprintf("%v", a)
		bs := fmt.Sprintf("%v", b)
		if as < bs {
			return -1
		}
		if as > bs {
			return 1
		}
		return 0
	}
}

func compareInts(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func toInt64(v any) int64 {
	switch tv := v.(type) {
	case int:
		return int64(tv)
	case int64:
		return tv
	case float64:
		return int64(tv)
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch tv := v.(type) {
	case float64:
		return tv
	case int:
		return float64(tv)
	case int64:
		return float64(tv)
	default:
		return 0
	}
}
