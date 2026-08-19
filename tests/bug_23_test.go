package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/scheduler"
)

type bug23Tx struct{}

func (bug23Tx) Commit(context.Context) error   { return nil }
func (bug23Tx) Rollback(context.Context) error { return nil }

type bug23Manager struct{}

func (bug23Manager) Begin(ctx context.Context) (context.Context, application.ScopeTransaction, error) {
	return ctx, bug23Tx{}, nil
}

type bug23Clock struct{ now time.Time }

func (c bug23Clock) Now() time.Time { return c.now }

type bug23Store struct {
	mu    sync.Mutex
	tasks []scheduler.Task
}

func (s *bug23Store) Load(context.Context) ([]scheduler.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]scheduler.Task(nil), s.tasks...)
	return out, nil
}

func (s *bug23Store) Save(_ context.Context, tasks []scheduler.Task) error {
	s.mu.Lock()
	s.tasks = append([]scheduler.Task(nil), tasks...)
	s.mu.Unlock()
	return nil
}

func TestBug23DeadlineStopsRetry(t *testing.T) {
	runner := application.NewScopeRunner(bug23Manager{})
	err := runner.Run(context.Background(), func(context.Context) error { return context.Canceled })
	if !application.IsExternalCancellation(err) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation identity was lost across the transaction boundary: %v", err)
	}

	clock := bug23Clock{now: time.Now()}
	store := &bug23Store{}
	var mu sync.Mutex
	attempts := 0
	started := make(chan struct{}, 4)
	handler := func(context.Context, scheduler.Task) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		started <- struct{}{}
		return err
	}
	s := scheduler.NewScheduler(clock, &scheduler.ConstantBackoff{Delay: time.Millisecond, AttemptLimit: 3}, store, handler)
	if err := s.Submit(context.Background(), scheduler.Task{ID: "deadline-23", NextRun: clock.Now(), Deadline: clock.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer s.Stop(context.Background())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not execute deadline task")
	}
	select {
	case <-started:
		t.Fatalf("deadline cancellation was retried; attempts=%d", attempts)
	case <-time.After(20 * time.Millisecond):
	}
}
