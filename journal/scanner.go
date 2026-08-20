package journal

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// SequenceContinues reports whether a record can follow the last record seen
// while scanning a directory of journal segments. Continuity is defined solely
// by the global sequence: the very first record of a replay must be sequence 1,
// and every subsequent record — whether it begins a new segment file or follows
// one within the same file — must advance the previous sequence by exactly one.
// File boundaries are intentionally irrelevant so that a log split across several
// segment files is recovered as a single continuous stream rather than treating
// each segment's first record as a fresh restart.
func SequenceContinues(previous, current uint64) bool {
	if previous == 0 {
		return current == 1
	}
	return current == previous+1
}

func scanJournal(file *os.File) (int64, uint64, error) {
	var offset int64
	var lastSeq uint64
	header := make([]byte, RecordHeaderSize)
	for {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return offset, lastSeq, err
		}
		n, err := io.ReadFull(file, header)
		if err != nil {
			if errors.Is(err, io.EOF) && n == 0 {
				return offset, lastSeq, nil
			}
			if errors.Is(err, io.ErrUnexpectedEOF) || (errors.Is(err, io.EOF) && n > 0) {
				return offset, lastSeq, ErrTailCorrupt
			}
			return offset, lastSeq, err
		}
		version := binary.BigEndian.Uint32(header[0:4])
		sequence := binary.BigEndian.Uint64(header[4:12])
		length := binary.BigEndian.Uint32(header[12:16])
		var checksum [32]byte
		copy(checksum[:], header[16:48])
		if version != CurrentVersion {
			return offset, lastSeq, ErrUnsupportedVersion
		}
		if length > MaxPayloadSize {
			return offset, lastSeq, ErrCorruptRecord
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(file, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return offset, lastSeq, ErrTailCorrupt
			}
			return offset, lastSeq, err
		}
		rec := Record{
			Version:  version,
			Sequence: sequence,
			Length:   length,
			Checksum: checksum,
			Payload:  payload,
		}
		if err := rec.Validate(); err != nil {
			nextOffset := offset + int64(RecordHeaderSize) + int64(length)
			if _, seekErr := file.Seek(nextOffset, io.SeekStart); seekErr != nil {
				return offset, lastSeq, err
			}
			var one [1]byte
			if _, readErr := file.Read(one[:]); readErr != nil {
				if errors.Is(readErr, io.EOF) {
					return offset, lastSeq, ErrTailCorrupt
				}
				return offset, lastSeq, err
			}
			return offset, lastSeq, ErrMidRecordCorruption
		}
		if sequence != lastSeq+1 {
			return offset, lastSeq, ErrInvalidSequence
		}
		lastSeq = sequence
		offset += int64(RecordHeaderSize) + int64(length)
	}
}
