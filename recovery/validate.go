package recovery

import "errors"

var (
	ErrInvalidChecksum   = errors.New("recovery: invalid checksum")
	ErrSequenceGap       = errors.New("recovery: sequence gap")
	ErrVersionRegression = errors.New("recovery: version regression")
)

func ValidateRecords(records []LogRecord, expectedVersion uint64) error {
	var lastSequence uint64
	for _, record := range records {
		if record.Version < expectedVersion {
			return ErrVersionRegression
		}
		if lastSequence != 0 && record.Sequence != lastSequence+1 {
			return ErrSequenceGap
		}
		if record.Sequence <= lastSequence {
			return ErrSequenceGap
		}
		if record.Checksum == 0 || record.Checksum != ComputeChecksum(record.Version, record.Sequence, record.Data) {
			return ErrInvalidChecksum
		}
		lastSequence = record.Sequence
	}
	return nil
}
