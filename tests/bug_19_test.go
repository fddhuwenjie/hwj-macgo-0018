package tests

import (
	"context"
	"hwj-macgo-0018/audit"
	"hwj-macgo-0018/domain"
	"testing"
	"time"
)

func TestBug19ConcurrentAuditDetailsIsolation(t *testing.T) {
	c, err := audit.Open("")
	if err != nil {
		t.Fatal(err)
	}
	details := map[string]string{"result": "one"}
	if _, err := c.Append(context.Background(), "worker", "commit", "result", "r1", details); err != nil {
		t.Fatal(err)
	}
	events := c.Events()
	events[0].Details["result"] = "tampered"
	if got := c.Events()[0].Details["result"]; got != "one" {
		t.Fatalf("audit details leaked through snapshot: %q", got)
	}
	if err := c.Verify(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.NewResultSnapshot("r1", "credential-1", "sha256:abcd", []byte("one"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	copy := snapshot.Clone()
	copy.Payload[0] = 'X'
	if string(snapshot.Payload) != "one" {
		t.Fatalf("result payload leaked through clone: %q", snapshot.Payload)
	}
}
