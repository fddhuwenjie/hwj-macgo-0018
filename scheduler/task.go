package scheduler

import (
	"context"
	"time"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID               string
	Generation       uint64
	Attempt          int
	NextRun          time.Time
	Deadline         time.Time
	Status           TaskStatus
	TerminationError string
	Payload          []byte
}

type Handler func(ctx context.Context, task Task) error

type TaskStore interface {
	Load(ctx context.Context) ([]Task, error)
	Save(ctx context.Context, tasks []Task) error
}

func (t Task) IsTerminal() bool {
	return t.Status == TaskCompleted || t.Status == TaskFailed || t.Status == TaskCancelled
}

func (t Task) IsLegalForRestore() bool {
	if t.ID == "" {
		return false
	}
	if t.Status == TaskCompleted || t.Status == TaskFailed || t.Status == TaskCancelled {
		return false
	}
	if t.NextRun.IsZero() {
		return false
	}
	return true
}
