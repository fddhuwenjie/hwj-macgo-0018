package tests

import (
	"context"
	"testing"
	"time"
)

func TestContextCancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	select {
	case <-ctx.Done():
	default:
		t.Fatal(`context cancellation was not propagated`)
	}
	if err := ctx.Err(); err != context.Canceled {
		t.Fatalf(`expected context.Canceled, got %v`, err)
	}
}

func TestTimeoutIsShortCircuitedByCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf(`expected deadline exceeded, got %v`, ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal(`timeout was not delivered`)
	}
}
