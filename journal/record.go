package journal

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
)

const (
	CurrentVersion   = 1
	RecordHeaderSize = 4 + 8 + 4 + 32
	MaxPayloadSize   = 16 * 1024 * 1024
)

type Record struct {
	Version  uint32
	Sequence uint64
	Length   uint32
	Checksum [32]byte
	Payload  []byte
}

func NewRecord(sequence uint64, payload []byte) (Record, error) {
	if len(payload) > MaxPayloadSize {
		return Record{}, ErrPayloadTooLarge
	}
	if sequence == 0 {
		return Record{}, ErrInvalidSequence
	}
	r := Record{
		Version:  CurrentVersion,
		Sequence: sequence,
		Length:   uint32(len(payload)),
		Payload:  append([]byte(nil), payload...),
	}
	r.Checksum = computeChecksum(r.Version, r.Sequence, r.Length, r.Payload)
	return r, nil
}

func (r Record) MarshalBinary() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	out := make([]byte, RecordHeaderSize+len(r.Payload))
	binary.BigEndian.PutUint32(out[0:4], r.Version)
	binary.BigEndian.PutUint64(out[4:12], r.Sequence)
	binary.BigEndian.PutUint32(out[12:16], r.Length)
	copy(out[16:48], r.Checksum[:])
	copy(out[48:], r.Payload)
	return out, nil
}

func UnmarshalRecord(data []byte) (Record, error) {
	if len(data) < RecordHeaderSize {
		return Record{}, io.ErrUnexpectedEOF
	}
	r := Record{
		Version:  binary.BigEndian.Uint32(data[0:4]),
		Sequence: binary.BigEndian.Uint64(data[4:12]),
		Length:   binary.BigEndian.Uint32(data[12:16]),
	}
	copy(r.Checksum[:], data[16:48])
	if r.Version != CurrentVersion {
		return Record{}, ErrUnsupportedVersion
	}
	if r.Length > MaxPayloadSize {
		return Record{}, ErrPayloadTooLarge
	}
	if uint32(len(data)-RecordHeaderSize) != r.Length {
		return Record{}, ErrCorruptRecord
	}
	r.Payload = make([]byte, r.Length)
	copy(r.Payload, data[RecordHeaderSize:])
	if err := r.Validate(); err != nil {
		return Record{}, err
	}
	return r, nil
}

func (r Record) Validate() error {
	if r.Version != CurrentVersion {
		return ErrUnsupportedVersion
	}
	if r.Sequence == 0 {
		return ErrInvalidSequence
	}
	if uint32(len(r.Payload)) != r.Length {
		return ErrCorruptRecord
	}
	if len(r.Payload) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	want := computeChecksum(r.Version, r.Sequence, r.Length, r.Payload)
	if r.Checksum != want {
		return ErrChecksumMismatch
	}
	return nil
}

func computeChecksum(version uint32, sequence uint64, length uint32, payload []byte) [32]byte {
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint32(buf[0:4], version)
	h.Write(buf[0:4])
	binary.BigEndian.PutUint64(buf[:8], sequence)
	h.Write(buf[:8])
	binary.BigEndian.PutUint32(buf[0:4], length)
	h.Write(buf[0:4])
	h.Write(payload)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
