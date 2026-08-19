package tests

import (
	"testing"
	"time"

	"hwj-macgo-0018/domain"
)

func TestBug04LateCallbackDiagnosis(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	key, err := domain.NewRequestKey("caller", "namespace", "key-4")
	if err != nil {
		t.Fatal(err)
	}
	r, err := domain.NewRequest("request-4", key, "digest-4", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionTo(domain.RequestStateOccupied, now); err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionTo(domain.RequestStateTakenOver, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	version := r.Version
	if err := r.TransitionTo(domain.RequestStateOccupied, now.Add(2*time.Minute)); err == nil {
		t.Fatalf("late callback returned request to occupied state at version %d", version)
	}
}
