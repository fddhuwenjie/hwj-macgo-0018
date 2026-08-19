package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hwj-macgo-0018/recovery"
)

func TestBug21RecoveryAcrossLogFiles(t *testing.T) {
	dir := t.TempDir()
	service := recovery.NewService(dir, nil, nil)
	if err := os.MkdirAll(service.LogDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	first := encodeRecoveryRecords(t, 1, 2, 3)
	second := encodeRecoveryRecords(t, 4, 5)
	firstPath := filepath.Join(service.LogDir(), "0001.log")
	secondPath := filepath.Join(service.LogDir(), "0002.log")
	if err := os.WriteFile(firstPath, first, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, second, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := service.Replay(context.Background())
	if err != nil {
		t.Fatalf("replay two contiguous segments: %v", err)
	}
	if result.Applied != 5 || result.LastSequence != 5 {
		t.Fatalf("expected all five records through sequence 5, got applied=%d last=%d", result.Applied, result.LastSequence)
	}
	if result.Truncated {
		t.Fatal("a segment beginning with the next global sequence must not be marked for truncation")
	}
	info, err := os.Stat(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(second)) {
		t.Fatalf("valid second segment changed from %d to %d bytes", len(second), info.Size())
	}

	// A later legal replay must still observe the same durable state.
	reopened := recovery.NewService(dir, nil, nil)
	again, err := reopened.Replay(context.Background())
	if err != nil {
		t.Fatalf("replay after reopen: %v", err)
	}
	if again.Applied != 5 || again.LastSequence != 5 || again.Truncated {
		t.Fatalf("reopened result changed: %+v", again)
	}

	thirdPath := filepath.Join(service.LogDir(), "0003.log")
	if err := os.WriteFile(thirdPath, encodeRecoveryRecords(t, 7), 0o644); err != nil {
		t.Fatal(err)
	}
	withGap, err := reopened.Replay(context.Background())
	if err != nil {
		t.Fatalf("detect sequence gap: %v", err)
	}
	if !withGap.Truncated || withGap.Applied != 5 || withGap.LastSequence != 5 {
		t.Fatalf("gap must preserve the five-record prefix: %+v", withGap)
	}
	if firstInfo, err := os.Stat(firstPath); err != nil || firstInfo.Size() != int64(len(first)) {
		t.Fatalf("first segment changed after gap: info=%v err=%v", firstInfo, err)
	}
	if secondInfo, err := os.Stat(secondPath); err != nil || secondInfo.Size() != int64(len(second)) {
		t.Fatalf("second segment changed after gap: info=%v err=%v", secondInfo, err)
	}

	if err := os.WriteFile(thirdPath, encodeRecoveryRecords(t, 6), 0o644); err != nil {
		t.Fatal(err)
	}
	afterCorrection, err := reopened.Replay(context.Background())
	if err != nil {
		t.Fatalf("replay corrected continuation: %v", err)
	}
	if afterCorrection.Applied != 6 || afterCorrection.LastSequence != 6 || afterCorrection.Truncated {
		t.Fatalf("legal continuation after rejected gap failed: %+v", afterCorrection)
	}
}

func encodeRecoveryRecords(t *testing.T, sequences ...uint64) []byte {
	t.Helper()
	var encoded []byte
	for _, sequence := range sequences {
		frame, err := recovery.EncodeLogRecord(recovery.LogRecord{
			Sequence: sequence,
			Version:  sequence,
			Data:     []byte{byte(sequence)},
		})
		if err != nil {
			t.Fatalf("encode sequence %d: %v", sequence, err)
		}
		encoded = append(encoded, frame...)
	}
	return encoded
}
