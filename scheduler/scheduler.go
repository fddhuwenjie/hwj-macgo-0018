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
		if len(s.tasks) == 0 {
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
		h := s.buildHeapLocked()
		if h.Len() == 0 {
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
	if err == nil {
		task.Status = TaskCompleted
		task.TerminationError = ""
		s.mu.Unlock()
		_ = s.persist(ctx)
		return nil
	}
	task.TerminationError = err.Error()
	now := s.clock.Now()
	if (!task.Deadline.IsZero() && !now.Before(task.Deadline)) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		task.Status = TaskCancelled
		s.mu.Unlock()
		_ = s.persist(ctx)
		return err
	}
	if s.retry.MaxAttempts() > 0 && task.Attempt >= s.retry.MaxAttempts() {
		task.Status = TaskFailed
		s.mu.Unlock()
		_ = s.persist(ctx)
		return nil
	}
	task.Status = TaskPending
	task.NextRun = s.clock.Now().Add(s.retry.NextDelay(task.Attempt))
	s.mu.Unlock()
	_ = s.persist(ctx)
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
		if task.IsLegalForRestore() {
			if task.Status == TaskRunning {
				task.Status = TaskPending
				task.Attempt++
				task.NextRun = s.clock.Now().Add(s.retry.NextDelay(task.Attempt))
			}
			cp := task
			s.tasks[task.ID] = &cp
		}
	}
	return nil
}

func (s *Scheduler) persist(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, *task)
	}
	return s.store.Save(ctx, tasks)
}
