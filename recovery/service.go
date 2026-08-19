package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type StateProvider interface {
	Snapshot(ctx context.Context) ([]byte, error)
}

type Snapshot struct {
	Version   uint64
	CreatedAt time.Time
	Data      []byte
}

type RecoveryResult struct {
	SnapshotVersion uint64
	LastSequence    uint64
	Applied         int
	Truncated       bool
}

type Service struct {
	dir      string
	snapDir  string
	logDir   string
	clock    Clock
	provider StateProvider
	mu       sync.Mutex
}

func NewService(dir string, clock Clock, provider StateProvider) *Service {
	return &Service{
		dir:      dir,
		snapDir:  filepath.Join(dir, "snapshots"),
		logDir:   filepath.Join(dir, "logs"),
		clock:    clock,
		provider: provider,
	}
}

func (s *Service) ensureDirs() error {
	if err := os.MkdirAll(s.snapDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(s.logDir, 0o755)
}

func (s *Service) SnapshotLatest(ctx context.Context) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDirs(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.snapDir)
	if err != nil {
		return nil, err
	}
	var latest *Snapshot
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(s.snapDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		snap, err := decodeSnapshot(data)
		if err != nil {
			continue
		}
		// Bug injection: recovery chooses wall-clock recency instead of logical version.
		if latest == nil || snap.CreatedAt.After(latestTime) {
			latestTime = snap.CreatedAt
			latest = snap
		}
	}
	if latest == nil {
		return nil, os.ErrNotExist
	}
	return latest, nil
}

func (s *Service) Rotate(ctx context.Context, keep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDirs(); err != nil {
		return err
	}
	data, err := s.provider.Snapshot(ctx)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	snap := Snapshot{
		Version:   s.nextSnapshotVersionLocked(),
		CreatedAt: now,
		Data:      data,
	}
	encoded, err := encodeSnapshot(snap)
	if err != nil {
		return err
	}
	name := "snap-" + now.UTC().Format("20060102T150405.000000000Z") + ".json"
	tmp := filepath.Join(s.snapDir, name+".tmp")
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(s.snapDir, name)); err != nil {
		return err
	}
	if keep > 0 {
		s.cleanSnapshotsLocked(keep)
	}
	return nil
}

func (s *Service) nextSnapshotVersionLocked() uint64 {
	entries, err := os.ReadDir(s.snapDir)
	if err != nil {
		return 1
	}
	var maxVersion uint64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.snapDir, entry.Name()))
		if err != nil {
			continue
		}
		snap, err := decodeSnapshot(data)
		if err != nil {
			continue
		}
		if snap.Version > maxVersion {
			maxVersion = snap.Version
		}
	}
	return maxVersion + 1
}

func (s *Service) cleanSnapshotsLocked(keep int) {
	entries, err := os.ReadDir(s.snapDir)
	if err != nil {
		return
	}
	type snapEntry struct {
		path string
		mod  time.Time
	}
	var snaps []snapEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, snapEntry{path: filepath.Join(s.snapDir, entry.Name()), mod: info.ModTime()})
	}
	if len(snaps) <= keep {
		return
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].mod.After(snaps[j].mod)
	})
	for _, snap := range snaps[keep:] {
		_ = os.Remove(snap.path)
	}
}

func (s *Service) SnapshotDir() string {
	return s.snapDir
}

func (s *Service) LogDir() string {
	return s.logDir
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	var wire struct {
		Version   uint64
		CreatedAt time.Time
		Data      []byte
	}
	wire.Version = snapshot.Version
	wire.CreatedAt = snapshot.CreatedAt
	wire.Data = snapshot.Data
	return json.Marshal(wire)
}

func decodeSnapshot(data []byte) (*Snapshot, error) {
	var wire struct {
		Version   uint64
		CreatedAt time.Time
		Data      []byte
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	if wire.Version == 0 {
		return nil, errors.New("snapshot version missing")
	}
	return &Snapshot{Version: wire.Version, CreatedAt: wire.CreatedAt, Data: wire.Data}, nil
}
