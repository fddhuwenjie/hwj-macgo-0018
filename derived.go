package query

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CurrentGenerationResult selects the result belonging to the request's generation.
func CurrentGenerationResult(requestGeneration, resultGeneration int64) bool {
	return requestGeneration == resultGeneration
}

type ConflictRequest struct {
	Caller    string
	Namespace string
	Key       string
	Count     int
}

type WaitDuration struct {
	ID       string
	Duration time.Duration
}

func (s *Service) ConflictRequests() []ConflictRequest {
	groups := make(map[string]int)
	for _, row := range s.rows {
		caller := stringField(row, "caller")
		namespace := stringField(row, "namespace")
		key := stringField(row, "key")
		if caller == "" || namespace == "" || key == "" {
			continue
		}
		groups[caller+"\x00"+namespace+"\x00"+key]++
	}
	out := make([]ConflictRequest, 0, len(groups))
	for k, count := range groups {
		parts := splitComposite(k)
		if len(parts) != 3 {
			continue
		}
		out = append(out, ConflictRequest{Caller: parts[0], Namespace: parts[1], Key: parts[2], Count: count})
	}
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

func splitComposite(s string) []string {
	return strings.Split(s, "\x00")
}
