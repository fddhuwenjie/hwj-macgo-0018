package journal

import "testing"

func FuzzUnmarshalRecord(f *testing.F) {
	record, err := NewRecord(1, []byte("seed"))
	if err == nil {
		if encoded, marshalErr := record.MarshalBinary(); marshalErr == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = UnmarshalRecord(data)
	})
}
