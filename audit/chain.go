package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

type Chain struct {
	mu       sync.Mutex
	file     *os.File
	events   []Event
	lastHash string
	now      func() time.Time
}

func Open(path string) (*Chain, error) {
	c := &Chain{now: time.Now}
	if path == "" {
		return c, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	c.file = f
	if err := c.load(); err != nil {
		f.Close()
		return nil, err
	}
	return c, nil
}

func (c *Chain) load() error {
	if c.file == nil {
		return nil
	}
	if _, err := c.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dec := json.NewDecoder(c.file)
	for {
		var e Event
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if e.PrevHash != c.lastHash {
			return ErrPrevHashMismatch
		}
		if err := e.Validate(); err != nil {
			return err
		}
		c.events = append(c.events, e)
		c.lastHash = e.Hash
	}
}

func (c *Chain) Append(ctx context.Context, actor, action, entityType, entityID string, details map[string]string) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	e := NewEvent(actor, action, entityType, entityID, details, c.lastHash, now())
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	if c.file != nil {
		data, err := json.Marshal(e)
		if err != nil {
			return Event{}, err
		}
		data = append(data, '\n')
		if _, err := c.file.Write(data); err != nil {
			return Event{}, err
		}
		if err := c.file.Sync(); err != nil {
			return Event{}, err
		}
	}
	c.events = append(c.events, e)
	c.lastHash = e.Hash
	return e, nil
}

func (c *Chain) Verify() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return verifyEvents(c.events)
}

func (c *Chain) LastHash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastHash
}

func (c *Chain) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func (c *Chain) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Event, len(c.events))
	for i, e := range c.events {
		e.Details = cloneDetails(e.Details)
		out[i] = e
	}
	return out
}

func (c *Chain) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

func verifyEvents(events []Event) error {
	var prev string
	for _, e := range events {
		if e.PrevHash != prev {
			return ErrPrevHashMismatch
		}
		if err := e.Validate(); err != nil {
			return err
		}
		prev = e.Hash
	}
	return nil
}
