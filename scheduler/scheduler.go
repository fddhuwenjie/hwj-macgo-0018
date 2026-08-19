package scheduler

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type taskHeap []*Task

func (h taskHeap) Len() int { return len(h) }
func (h taskHeap) Less(i, j int) bool {
	return h[i].NextRun.Before(h[j].NextRun)
}
func (h taskHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *taskHeap) Push(x interface{}) { *h = append(*h, x.(*Task)) }
func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

type Scheduler struct {
	clock   Clock
	retry   RetryPolicy
	store   TaskStore
	handler Handler

	mu    sync.Mutex
	tasks map[string]*Task
	stop  chan struct{}
	wg    sync.WaitGroup
}

func NewScheduler(clock Clock, retry RetryPolicy, store TaskStore, handler Handler) *Scheduler {
	return &Scheduler{
		clock:   clock,
		retry:   retry,
		store:   store,
		handler: handler,
		tasks:   make(map[string]*Task),
		stop:    make(chan struct{}),
	}
}

func (s *Scheduler) Submit(ctx context.Context, task Task) error {
	if task.ID == "" {
		return errors.New("scheduler: task id is required")
	}
	if task.NextRun.IsZero() {
		task.NextRun = s.clock.Now()
	}
	if task.Status == "" {
		task.Status = TaskPending
	}
	s.mu.Lock()
	s.tasks[task.ID] = &task
	s.mu.Unlock()
	return s.persist(ctx)
}

func (s *Scheduler) Start(ctx context.Context) error {
	if s.handler == nil {
		return errors.New("scheduler: handler is required")
	}
	if err := s.restore(ctx); err != nil {
		return err
	}
	s.wg.Add(1)
	go s.loop(ctx)
	return nil
}

func (s *Scheduler) Stop(ctx context.Context) error {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()
	for {
		s.mu.Lock()
		h := s.buildHeapLocked()
		if h.Len() == 0 {
			// Nothing runnable: no tasks at all, or every loaded task has
			// reached a terminal state. Idle rather than indexing an empty
			// heap, so completion does not panic the loop and new submissions
			// can still be picked up on the next tick.
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-time.After(time.Second):
				continue
			}
		}
		next := h[0].NextRun
		s.mu.Unlock()
		delay := time.Until(next)
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.stop:
			timer.Stop()
			return
		case <-timer.C:
			s.runDue(ctx)
		}
	}
}

func (s *Scheduler) runDue(ctx context.Context) {
	s.mu.Lock()
	h := s.buildHeapLocked()
	now := s.clock.Now()
	var due *Task
	for h.Len() > 0 {
		t := heap.Pop(&h).(*Task)
		if t.NextRun.After(now) {
			continue
		}
		due = t
		break
	}
	s.mu.Unlock()
	if due == nil {
		return
	}
	_ = s.execute(ctx, due)
}

func (s *Scheduler) buildHeapLocked() taskHeap {
	h := make(taskHeap, 0, len(s.tasks))
	for _, task := range s.tasks {
		if task.IsTerminal() {
			continue
		}
		h = append(h, task)
	}
	heap.Init(&h)
	return h
}

func (s *Scheduler) execute(ctx context.Context, task *Task) error {
	s.mu.Lock()
	task.Status = TaskRunning
	task.Attempt++
	if task.Deadline.IsZero() {
		task.Deadline = s.clock.Now().Add(time.Minute)
	}
	s.mu.Unlock()
	_ = s.persist(ctx)

	err := s.handler(ctx, *task)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		task.Status = TaskCompleted
		task.TerminationError = ""
		_ = s.persistLocked(ctx)
		return nil
	}
	task.TerminationError = err.Error()
	if s.retry.MaxAttempts() > 0 && task.Attempt >= s.retry.MaxAttempts() {
		task.Status = TaskFailed
		_ = s.persistLocked(ctx)
		return nil
	}
	task.Status = TaskPending
	task.NextRun = s.clock.Now().Add(s.retry.NextDelay(task.Attempt))
	_ = s.persistLocked(ctx)
	return err
}

func (s *Scheduler) restore(ctx context.Context) error {
	tasks, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range tasks {
		if !task.IsLegalForRestore() {
			continue
		}
		if task.Status == TaskRunning {
			// A task left running by a previous process is an abandoned
			// execution. Recovery claims it for a fresh execution: advance
			// the execution generation (not merely the retry attempt) and
			// schedule an immediate re-run. The new generation starts its own
			// attempt counter, so the abandoned execution's attempts do not
			// carry over and no backoff delay applies to a first attempt.
			task.Status = TaskPending
			task.Generation++
			task.Attempt = 0
			task.NextRun = s.clock.Now()
		}
		cp := task
		s.tasks[task.ID] = &cp
	}
	// Make the recovered generation durable so a crash between recovery and
	// execution cannot lose the advance. If persistence fails, roll the
	// in-memory state back to the original snapshots so the store stays
	// consistent with what was loaded and a later restart can retry recovery
	// against an uncorrupted task.
	if err := s.persistLocked(ctx); err != nil {
		s.tasks = make(map[string]*Task)
		for _, task := range tasks {
			if task.IsLegalForRestore() {
				cp := task
				s.tasks[task.ID] = &cp
			}
		}
		return err
	}
	return nil
}

func (s *Scheduler) persist(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked(ctx)
}

// persistLocked snapshots the task map and writes it through the store
// without taking s.mu. Callers that already hold s.mu must use this entry
// point; taking s.mu again here would self-deadlock the completion write-back.
func (s *Scheduler) persistLocked(ctx context.Context) error {
	tasks := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, *task)
	}
	return s.store.Save(ctx, tasks)
}
