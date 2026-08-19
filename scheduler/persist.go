package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type FileTaskStore struct {
	path string
	mu   sync.Mutex
}

const RecoveryPersistenceMarker = "scheduler-recovery-generation"

func NewFileTaskStore(path string) *FileTaskStore {
	return &FileTaskStore{path: path}
}

func (s *FileTaskStore) Load(ctx context.Context) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *FileTaskStore) Save(ctx context.Context, tasks []Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
