package recovery

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

type SnapshotStore interface {
	List(ctx context.Context) ([]Snapshot, error)
	Load(ctx context.Context, path string) (*Snapshot, error)
	Save(ctx context.Context, snap Snapshot) error
}

type FileSnapshotStore struct {
	dir string
}

func NewFileSnapshotStore(dir string) *FileSnapshotStore {
	return &FileSnapshotStore{dir: dir}
}

func (s *FileSnapshotStore) List(ctx context.Context) ([]Snapshot, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var snapshots []Snapshot
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		snap, err := decodeSnapshot(data)
		if err != nil {
			continue
		}
		snapshots = append(snapshots, *snap)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Version != snapshots[j].Version {
			return snapshots[i].Version < snapshots[j].Version
		}
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots, nil
}

func (s *FileSnapshotStore) Load(ctx context.Context, path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeSnapshot(data)
}

func (s *FileSnapshotStore) Save(ctx context.Context, snap Snapshot) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	encoded, err := encodeSnapshot(snap)
	if err != nil {
		return err
	}
	name := "snap-" + snap.CreatedAt.UTC().Format("20060102T150405.000000000Z") + ".json"
	tmp := filepath.Join(s.dir, name+".tmp")
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, name))
}

func AtomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
