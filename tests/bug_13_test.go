package tests

import (
	"hwj-macgo-0018/domain"
	"testing"
	"time"
)

func TestBug13FutureLeaseRemainsUsable(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	lease, err := domain.NewLease("lease-13", "cred-13", "worker", now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if lease.IsExpired(now) {
		t.Fatal("future lease was treated as expired")
	}
	if lease.IsExpired(now.Add(30 * time.Minute)) {
		t.Fatal("future lease expired before its deadline")
	}
}

func TestBug13LeaseBoundaryIsExpiredAtDeadline(t *testing.T) {
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	lease, err := domain.NewLease("lease-13b", "cred-13b", "worker", now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	// At the exact deadline the lease is expired: the deadline instant is never
	// treated as still usable.
	if !lease.IsExpired(now.Add(time.Hour)) {
		t.Fatal("lease was not expired at its deadline")
	}
	// After the deadline it stays expired.
	if !lease.IsExpired(now.Add(2 * time.Hour)) {
		t.Fatal("lease was not expired after its deadline")
	}
}
