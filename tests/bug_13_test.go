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
