package audit

import (
	"encoding/json"
	"errors"
	"io"
)

func (c *Chain) Export(w io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := verifyEvents(c.events); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	for _, e := range c.events {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func VerifyExport(r io.Reader) ([]Event, error) {
	dec := json.NewDecoder(r)
	var events []Event
	var prev string
	for {
		var e Event
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return nil, err
		}
		if e.PrevHash != prev {
			return nil, ErrPrevHashMismatch
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		events = append(events, e)
		prev = e.Hash
	}
}
