package journal

import "errors"

var (
	ErrPayloadTooLarge     = errors.New("journal: payload too large")
	ErrUnsupportedVersion  = errors.New("journal: unsupported record version")
	ErrCorruptRecord       = errors.New("journal: corrupt record")
	ErrChecksumMismatch    = errors.New("journal: record checksum mismatch")
	ErrInvalidSequence     = errors.New("journal: invalid sequence")
	ErrJournalClosed       = errors.New("journal: journal closed")
	ErrAppendCanceled      = errors.New("journal: append canceled")
	ErrTailCorrupt         = errors.New("journal: tail corrupt")
	ErrMidRecordCorruption = errors.New("journal: mid-record corruption")
)
