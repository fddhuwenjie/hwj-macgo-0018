package tests

import (
	"context"
	"hwj-macgo-0018/audit"
	"path/filepath"
	"testing"
)

func TestBug10AuditChainReopenDiagnosis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	ctx := context.Background()
	c, err := audit.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Append(ctx, "u", "create", "lease", "1", map[string]string{"step": "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Append(ctx, "u", "occupy", "lease", "1", map[string]string{"step": "two"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := audit.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Append(ctx, "u", "commit", "lease", "1", map[string]string{"step": "three"}); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Verify(); err == nil {
		t.Fatal("diagnosis expected reopened audit chain to expose the broken predecessor")
	}
}
