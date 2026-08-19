package recovery

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type LogRecord struct {
	Sequence uint64
	Version  uint64
	Length   uint32
	Checksum uint32
	Data     []byte
}

func ComputeChecksum(version uint64, sequence uint64, data []byte) uint32 {
	h := crc32.NewIEEE()
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], version)
	binary.BigEndian.PutUint64(buf[8:16], sequence)
	_, _ = h.Write(buf[:])
	_, _ = h.Write(data)
	return h.Sum32()
}

func EncodeLogRecord(record LogRecord) ([]byte, error) {
	if record.Length == 0 {
		record.Length = uint32(len(record.Data))
	}
	if record.Checksum == 0 {
		record.Checksum = ComputeChecksum(record.Version, record.Sequence, record.Data)
	}
	var header [24]byte
	binary.BigEndian.PutUint64(header[0:8], record.Sequence)
	binary.BigEndian.PutUint64(header[8:16], record.Version)
	binary.BigEndian.PutUint32(header[16:20], record.Length)
	binary.BigEndian.PutUint32(header[20:24], record.Checksum)
	out := make([]byte, 0, 24+int(record.Length))
	out = append(out, header[:]...)
	out = append(out, record.Data...)
	return out, nil
}

func DecodeLogRecord(frame []byte) (LogRecord, error) {
	if len(frame) < 24 {
		return LogRecord{}, errors.New("recovery: log record too short")
	}
	sequence := binary.BigEndian.Uint64(frame[0:8])
	version := binary.BigEndian.Uint64(frame[8:16])
	length := binary.BigEndian.Uint32(frame[16:20])
	checksum := binary.BigEndian.Uint32(frame[20:24])
	if len(frame) != 24+int(length) {
		return LogRecord{}, errors.New("recovery: log record length mismatch")
	}
	data := make([]byte, length)
	copy(data, frame[24:])
	want := ComputeChecksum(version, sequence, data)
	if want != checksum {
		return LogRecord{}, ErrInvalidChecksum
	}
	return LogRecord{
		Sequence: sequence,
		Version:  version,
		Length:   length,
		Checksum: checksum,
		Data:     data,
	}, nil
}

func (s *Service) Replay(ctx context.Context) (*RecoveryResult, error) {
	if err := s.ensureDirs(); err != nil {
		return nil, err
	}
	snapshot, err := s.SnapshotLatest(ctx)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	result := &RecoveryResult{}
	if snapshot != nil {
		result.SnapshotVersion = snapshot.Version
		result.LastSequence = snapshot.Version
	}
	logs, err := s.listLogFiles()
	if err != nil {
		return nil, err
	}
	sort.Strings(logs)
	for _, logPath := range logs {
		applied, truncated, err := s.replayFile(logPath, snapshot, result)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		result.Applied += applied
		result.Truncated = result.Truncated || truncated
	}
	return result, nil
}

func (s *Service) Truncate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDirs(); err != nil {
		return err
	}
	logs, err := s.listLogFiles()
	if err != nil {
		return err
	}
	var lastErr error
	for _, logPath := range logs {
		if err := s.truncateFile(logPath); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *Service) listLogFiles() ([]string, error) {
	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(s.logDir, entry.Name()))
	}
	return paths, nil
}

func (s *Service) replayFile(path string, snapshot *Snapshot, result *RecoveryResult) (int, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var applied int
	truncated := false
	for {
		header := make([]byte, 24)
		n, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || n != 24 {
			truncated = true
			break
		}
		length := binary.BigEndian.Uint32(header[16:20])
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			truncated = true
			break
		}
		frame := append(header, data...)
		rec, err := DecodeLogRecord(frame)
		if err != nil {
			truncated = true
			break
		}
		if snapshot != nil && rec.Version < snapshot.Version {
			continue
		}
		if result.LastSequence != 0 && rec.Sequence != result.LastSequence+1 {
			truncated = true
			break
		}
		if rec.Sequence == 0 {
			truncated = true
			break
		}
		result.LastSequence = rec.Sequence
		applied++
	}
	if truncated {
		if err := s.truncateFile(path); err != nil {
			return applied, true, err
		}
	}
	return applied, truncated, nil
}

func (s *Service) truncateFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var goodPos int64
	for {
		header := make([]byte, 24)
		_, err := io.ReadFull(reader, header)
		if err != nil {
			break
		}
		length := binary.BigEndian.Uint32(header[16:20])
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			break
		}
		frame := append(header, data...)
		if _, err := DecodeLogRecord(frame); err != nil {
			break
		}
		goodPos += int64(len(frame))
	}
	return file.Truncate(goodPos)
}
