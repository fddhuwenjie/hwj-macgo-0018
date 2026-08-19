package tests

import (
	"context"
	"hwj-macgo-0018/scheduler"
	"sync"
	"testing"
	"time"
)

type bug11Store struct{ tasks []scheduler.Task }

func (s *bug11Store) Load(context.Context) ([]scheduler.Task, error) { return s.tasks, nil }
func (s *bug11Store) Save(_ context.Context, tasks []scheduler.Task) error {
	s.tasks = append([]scheduler.Task(nil), tasks...)
	return nil
}

type bug11Clock struct{ now time.Time }

func (c bug11Clock) Now() time.Time { return c.now }

func TestBug11SchedulerRecoveryDuplicateRun(t *testing.T) {
	now := time.Now()
	store := &bug11Store{tasks: []scheduler.Task{{ID: "task-11", Generation: 4, Attempt: 1, Status: scheduler.TaskRunning, NextRun: now}}}
	var mu sync.Mutex
	var got scheduler.Task
	done := make(chan struct{})
	s := scheduler.NewScheduler(bug11Clock{now: now}, &scheduler.ConstantBackoff{Delay: time.Millisecond, AttemptLimit: 1}, store, func(_ context.Context, task scheduler.Task) error {
		mu.Lock()
		got = task
		mu.Unlock()
		close(done)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("recovered task did not run")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer stopCancel()
	if err := s.Stop(stopCtx); err != nil {
		t.Fatalf("scheduler did not stop cleanly after recovered execution: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got.Generation <= 4 {
		t.Fatalf("recovery reused generation %d; expected a new execution generation", got.Generation)
	}
}
