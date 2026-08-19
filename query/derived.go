package query

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

type ConflictRequest struct {
	Caller     string
	Namespace  string
	Key        string
	Count      int
	Generation uint64
	Reasons    []string
}

type WaitDuration struct {
	ID       string
	Duration time.Duration
}

func (s *Service) ConflictRequests() []ConflictRequest {
	type group struct {
		caller     string
		namespace  string
		key        string
		generation uint64
		count      int
		reasons    []string
		seen       map[string]struct{}
	}
	// Group by caller+namespace+key+generation so each execution generation
	// survives aggregation as its own row instead of being merged into a
	// single count. The generation is carried by the failure record and must
	// be threaded through here; otherwise two failures from different
	// generations collapse into one aggregate and the per-generation reason
	// is lost, even after the store is reopened.
	groups := make(map[string]*group)
	for _, row := range s.rows {
		caller := stringField(row, "caller")
		namespace := stringField(row, "namespace")
		key := stringField(row, "key")
		if caller == "" || namespace == "" || key == "" {
			continue
		}
		generation := uint64Field(row, "generation")
		gk := caller + "\x00" + namespace + "\x00" + key + "\x00" + strconv.FormatUint(generation, 10)
		g, ok := groups[gk]
		if !ok {
			g = &group{
				caller:     caller,
				namespace:  namespace,
				key:        key,
				generation: generation,
				seen:       make(map[string]struct{}),
			}
			groups[gk] = g
		}
		g.count++
		if reason := stringField(row, "reason"); reason != "" {
			if _, dup := g.seen[reason]; !dup {
				g.seen[reason] = struct{}{}
				g.reasons = append(g.reasons, reason)
			}
		}
	}
	out := make([]ConflictRequest, 0, len(groups))
	for _, g := range groups {
		out = append(out, ConflictRequest{
			Caller:     g.caller,
			Namespace:  g.namespace,
			Key:        g.key,
			Count:      g.count,
			Generation: g.generation,
			Reasons:    g.reasons,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Caller != out[j].Caller {
			return out[i].Caller < out[j].Caller
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Generation < out[j].Generation
	})
	return out
}

func (s *Service) HangingExecutions(now time.Time) []Row {
	var out []Row
	for _, row := range s.rows {
		if stringField(row, "status") != "executing" {
			continue
		}
		expires, ok := row.Fields["expires_at"]
		if !ok {
			continue
		}
		t, ok := expires.(time.Time)
		if !ok {
			if str, ok := expires.(string); ok {
				parsed, err := time.Parse(time.RFC3339, str)
				if err != nil {
					continue
				}
				t = parsed
			} else {
				continue
			}
		}
		if now.After(t) {
			out = append(out, row)
		}
	}
	return out
}

func (s *Service) ReplayHitRate() float64 {
	if len(s.rows) == 0 {
		return 0
	}
	hits := 0
	for _, row := range s.rows {
		if boolField(row, "replay_hit") {
			hits++
		}
	}
	return float64(hits) / float64(len(s.rows))
}

func (s *Service) WaitDurations() []WaitDuration {
	out := make([]WaitDuration, 0, len(s.rows))
	for _, row := range s.rows {
		ms, ok := row.Fields["wait_ms"]
		if !ok {
			continue
		}
		var d time.Duration
		switch v := ms.(type) {
		case int:
			d = time.Duration(v) * time.Millisecond
		case int64:
			d = time.Duration(v) * time.Millisecond
		case float64:
			d = time.Duration(v) * time.Millisecond
		case string:
			parsed, err := time.ParseDuration(v)
			if err != nil {
				continue
			}
			d = parsed
		default:
			continue
		}
		out = append(out, WaitDuration{ID: row.ID, Duration: d})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Duration == out[j].Duration {
			return out[i].ID < out[j].ID
		}
		return out[i].Duration < out[j].Duration
	})
	return out
}

func stringField(row Row, key string) string {
	v, ok := row.Fields[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func boolField(row Row, key string) bool {
	v, ok := row.Fields[key]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func uint64Field(row Row, key string) uint64 {
	v, ok := row.Fields[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case uint64:
		return n
	case uint:
		return uint64(n)
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case float64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case string:
		parsed, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
