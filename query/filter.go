package query

import (
	"fmt"
	"strings"
)

func All() Predicate {
	return func(Row) bool { return true }
}

func Any(predicates ...Predicate) Predicate {
	return func(row Row) bool {
		for _, p := range predicates {
			if p == nil {
				continue
			}
			if p(row) {
				return true
			}
		}
		return false
	}
}

func Not(predicate Predicate) Predicate {
	return func(row Row) bool {
		if predicate == nil {
			return true
		}
		return !predicate(row)
	}
}

func FieldEquals(key string, value any) Predicate {
	return func(row Row) bool {
		v, ok := row.Fields[key]
		return ok && equalValues(v, value)
	}
}

func FieldNotEquals(key string, value any) Predicate {
	return Not(FieldEquals(key, value))
}

func FieldContains(key string, substr string) Predicate {
	return func(row Row) bool {
		v, ok := row.Fields[key]
		if !ok {
			return false
		}
		str, ok := v.(string)
		if !ok {
			str = fmt.Sprintf("%v", v)
		}
		return strings.Contains(str, substr)
	}
}

func HasField(key string) Predicate {
	return func(row Row) bool {
		_, ok := row.Fields[key]
		return ok
	}
}

func equalValues(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
