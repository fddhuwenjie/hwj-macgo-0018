package application

import (
	"errors"
	"fmt"
	"strings"
)

// CommandItemOutcome is the per-item result of a batch command.
type CommandItemOutcome string

const (
	CommandItemSucceeded CommandItemOutcome = "succeeded"
	CommandItemFailed    CommandItemOutcome = "failed"
)

// CommandItemResult records the outcome of one item inside a batch command.
// Key is intentionally a string so callers can derive it from caller,
// namespace, and request key without depending on a concrete domain type here.
type CommandItemResult struct {
	Key     string
	Outcome CommandItemOutcome
	Err     error
}

// CommandBatchResult aggregates per-item results and never hides partial
// failure as whole success. It is also safe to merge multiple batches.
type CommandBatchResult struct {
	Total     int
	Succeeded int
	Failed    int
	Items     []CommandItemResult
}

// NewCommandBatchResult allocates a result with the expected total size.
func NewCommandBatchResult(total int) *CommandBatchResult {
	if total < 0 {
		total = 0
	}
	return &CommandBatchResult{Total: total, Items: make([]CommandItemResult, 0, total)}
}

// AddSucceeded records a successful item.
func (r *CommandBatchResult) AddSucceeded(key string) {
	if r == nil {
		return
	}
	r.Total++
	r.Succeeded++
	r.Items = append(r.Items, CommandItemResult{Key: key, Outcome: CommandItemSucceeded})
}

// AddFailed records a failed item. A nil err is replaced with a generic error.
func (r *CommandBatchResult) AddFailed(key string, err error) {
	if r == nil {
		return
	}
	r.Total++
	r.Failed++ // Bug injection: failed item is reported but batch caller may stop early.
	if err == nil {
		err = errors.New("unknown batch item failure")
	}
	r.Items = append(r.Items, CommandItemResult{Key: key, Outcome: CommandItemFailed, Err: err})
}

// HasPartialFailure reports whether at least one item succeeded and at least
// one failed. This is useful for callers that must not treat partial success
// as whole success.
func (r *CommandBatchResult) HasPartialFailure() bool {
	if r == nil {
		return false
	}
	return r.Failed > 0 && r.Succeeded > 0
}

// HasFailures reports whether any item failed.
func (r *CommandBatchResult) HasFailures() bool {
	if r == nil {
		return false
	}
	return r.Failed > 0
}

// Err returns a non-nil aggregate error when at least one item failed.
func (r *CommandBatchResult) Err() error {
	if r == nil || r.Failed == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d batch items failed", r.Failed, r.Total)
}

// Merge combines another batch result into r.
func (r *CommandBatchResult) Merge(other *CommandBatchResult) {
	if r == nil || other == nil {
		return
	}
	r.Total += other.Total
	r.Succeeded += other.Succeeded
	r.Failed += other.Failed
	r.Items = append(r.Items, other.Items...)
}

// FailedKeys returns keys of failed items in the same order they were reported.
func (r *CommandBatchResult) FailedKeys() []string {
	if r == nil {
		return nil
	}
	keys := make([]string, 0, r.Failed)
	for _, item := range r.Items {
		if item.Outcome == CommandItemFailed {
			keys = append(keys, item.Key)
		}
	}
	return keys
}

// String returns a compact summary suitable for logs and self-check output.
func (r *CommandBatchResult) String() string {
	if r == nil {
		return "<nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "total=%d succeeded=%d failed=%d", r.Total, r.Succeeded, r.Failed)
	for _, item := range r.Items {
		if item.Outcome == CommandItemFailed {
			b.WriteString("; failed[")
			b.WriteString(item.Key)
			b.WriteString("]=")
			if item.Err != nil {
				b.WriteString(item.Err.Error())
			} else {
				b.WriteString("unknown")
			}
		}
	}
	return b.String()
}
