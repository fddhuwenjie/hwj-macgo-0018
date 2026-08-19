package tests

import (
	"context"
	"hwj-macgo-0018/scheduler"
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
	s := scheduler.NewScheduler(bug11Clock{now: now}, &scheduler.ConstantBackoff{Delay: time.Millisecond, AttemptLimit: 1}, store, nil)
	if err := s.RestoreForInspection(context.Background()); err != nil {
		t.Fatal(err)
	}
	tasks := s.SnapshotTasks()
	if len(tasks) != 1 || tasks[0].Generation <= 4 {
		t.Fatalf("recovery reused generation: %#v", tasks)
	}
}
