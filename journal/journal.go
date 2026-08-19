package journal

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
)

// IsSnapshotCandidate reports whether a directory entry is a committed
// snapshot that may participate in recovery and version selection. A snapshot
// is committed only once its temp file has been renamed into place, so the
// trailing ".json" suffix is required: a leftover "snap-*.json.tmp" from a
// failed or interrupted write must never be treated as a recovery point.
func IsSnapshotCandidate(name string) bool {
	return strings.HasPrefix(name, "snap-") && strings.HasSuffix(name, ".json")
}

type Journal struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	sequence uint64
	closed   bool
}

func Open(path string) (*Journal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	j := &Journal{file: f, path: path}
	if err := j.recover(); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

func (j *Journal) recover() error {
	validOffset, lastSeq, err := scanJournal(j.file)
	if err != nil {
		if errors.Is(err, ErrTailCorrupt) {
			if err := j.file.Truncate(validOffset); err != nil {
				return err
			}
			if _, err := j.file.Seek(validOffset, io.SeekStart); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if _, err := j.file.Seek(validOffset, io.SeekStart); err != nil {
			return err
		}
	}
	j.sequence = lastSeq
	return nil
}

func (j *Journal) Append(ctx context.Context, payload []byte) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return Record{}, ErrJournalClosed
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	if len(payload) > MaxPayloadSize {
		return Record{}, ErrPayloadTooLarge
	}
	pos, err := j.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return Record{}, err
	}
	j.sequence++
	rec, err := NewRecord(j.sequence, payload)
	if err != nil {
		j.sequence--
		return Record{}, err
	}
	data, err := rec.MarshalBinary()
	if err != nil {
		j.sequence--
		return Record{}, err
	}
	if _, err := j.file.Write(data); err != nil {
		_ = j.file.Truncate(pos)
		_, _ = j.file.Seek(pos, io.SeekStart)
		j.sequence--
		return Record{}, err
	}
	if err := j.file.Sync(); err != nil {
		_ = j.file.Truncate(pos)
		_, _ = j.file.Seek(pos, io.SeekStart)
		j.sequence--
		return Record{}, err
	}
	return rec, nil
}

func (j *Journal) LastSequence() uint64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.sequence
}

func (j *Journal) Sync() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return ErrJournalClosed
	}
	return j.file.Sync()
}

func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	return j.file.Close()
}
