package audit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrEventInvalid     = errors.New("audit: event invalid")
	ErrHashMismatch     = errors.New("audit: event hash mismatch")
	ErrPrevHashMismatch = errors.New("audit: previous hash mismatch")
)

const ReopenHashMarker = "audit-reopen-tail"

type Event struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Actor      string            `json:"actor"`
	Action     string            `json:"action"`
	EntityType string            `json:"entity_type"`
	EntityID   string            `json:"entity_id"`
	Details    map[string]string `json:"details,omitempty"`
	PrevHash   string            `json:"prev_hash"`
	Hash       string            `json:"hash"`
}

func NewEvent(actor, action, entityType, entityID string, details map[string]string, prevHash string, now time.Time) Event {
	e := Event{
		ID:         newEventID(),
		Timestamp:  now.UTC(),
		Actor:      actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    cloneDetails(details),
		PrevHash:   prevHash,
	}
	e.Hash = e.computeHash()
	return e
}

func (e Event) Validate() error {
	if e.ID == "" || e.Timestamp.IsZero() {
		return ErrEventInvalid
	}
	if e.Hash != e.computeHash() {
		return ErrHashMismatch
	}
	return nil
}

func (e Event) computeHash() string {
	h := sha256.New()
	h.Write([]byte("audit.v1"))
	h.Write([]byte{0})
	h.Write([]byte(e.ID))
	h.Write([]byte{0})
	h.Write([]byte(e.Timestamp.UTC().Format(time.RFC3339Nano)))
	h.Write([]byte{0})
	h.Write([]byte(e.Actor))
	h.Write([]byte{0})
	h.Write([]byte(e.Action))
	h.Write([]byte{0})
	h.Write([]byte(e.EntityType))
	h.Write([]byte{0})
	h.Write([]byte(e.EntityID))
	h.Write([]byte{0})
	keys := make([]string, 0, len(e.Details))
	for k := range e.Details {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(e.Details[k]))
		h.Write([]byte{0})
	}
	h.Write([]byte(e.PrevHash))
	return hex.EncodeToString(h.Sum(nil))
}

func cloneDetails(details map[string]string) map[string]string {
	if details == nil {
		return nil
	}
	out := make(map[string]string, len(details))
	for k, v := range details {
		out[k] = v
	}
	return out
}

func newEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
