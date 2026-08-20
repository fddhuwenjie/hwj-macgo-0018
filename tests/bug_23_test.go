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

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

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
		t.Errorf("cancellation identity was lost across the transaction boundary: %v", err)
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
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Stop(stopCtx)
	}()
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

func TestBug23CancelledTaskNotResumedAfterRestart(t *testing.T) {
	clock := realClock{}
	cancelledErr := application.NewAppExecError(application.AppExecErrorKindCancelled, "test", context.Canceled)
	store := &bug23Store{}
	crashState := []scheduler.Task{
		{ID: "deadline-23", Attempt: 2, NextRun: clock.Now().Add(time.Second), Deadline: clock.Now().Add(-time.Second), Status: scheduler.TaskCancelled, TerminationError: cancelledErr.Error()},
		{ID: "inflight-23", Attempt: 3, NextRun: clock.Now(), Deadline: clock.Now().Add(time.Minute), Status: scheduler.TaskRunning},
	}
	if err := store.Save(context.Background(), crashState); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	runs := map[string]int{}
	startedC := make(chan string, 8)
	handler := func(_ context.Context, task scheduler.Task) error {
		mu.Lock()
		runs[task.ID]++
		mu.Unlock()
		startedC <- task.ID
		return cancelledErr
	}
	s := scheduler.NewScheduler(clock, &scheduler.ConstantBackoff{Delay: time.Millisecond, AttemptLimit: 3}, store, handler)
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Stop(stopCtx)
	}()
	select {
	case id := <-startedC:
		if id != "inflight-23" {
			t.Errorf("cancelled task was restored and re-run after restart: %s", id)
		}
	case <-time.After(time.Second):
		t.Error("in-flight task was not restored after restart")
	}
	select {
	case id := <-startedC:
		t.Errorf("stopped task ran an extra time after restart: %s", id)
	case <-time.After(30 * time.Millisecond):
	}
	mu.Lock()
	defer mu.Unlock()
	if runs["deadline-23"] != 0 {
		t.Errorf("cancelled task was retried after restart: %d runs", runs["deadline-23"])
	}
	if runs["inflight-23"] == 0 {
		t.Error("in-flight task was not re-run after restart")
	}
	if runs["inflight-23"] != 1 {
		t.Errorf("in-flight task ran %d times after restart, want 1", runs["inflight-23"])
	}
}
