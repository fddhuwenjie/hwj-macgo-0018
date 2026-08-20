package tests

import (
	"context"
	"testing"

	"hwj-macgo-0018/audit"
)

func TestBug10AuditChainReopenDiagnosis(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/audit.jsonl"
	chain, err := audit.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := chain.Append(ctx, "worker", "create", "execution", "task-10", map[string]string{"step": "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := chain.Append(ctx, "worker", "occupy", "execution", "task-10", map[string]string{"step": "two"})
	if err != nil {
		t.Fatal(err)
	}
	if second.PrevHash != first.Hash || chain.LastHash() != second.Hash {
		t.Fatal("initial audit prefix was not constructed correctly")
	}
	if err := chain.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := chain.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := audit.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.LastHash(); got != second.Hash {
		t.Errorf("reopen restored tail %s, want second event %s", got, second.Hash)
	}
	third, err := reopened.Append(ctx, "worker", "commit", "execution", "task-10", map[string]string{"step": "three"})
	if err != nil {
		t.Fatal(err)
	}
	if third.PrevHash != second.Hash {
		t.Errorf("first append after reopen linked to %s, want %s", third.PrevHash, second.Hash)
	}
	if err := reopened.Verify(); err != nil {
		t.Errorf("chain became invalid after first reopened append: %v", err)
	}

	fourth, err := reopened.Append(ctx, "worker", "publish", "execution", "task-10", map[string]string{"step": "four"})
	if err != nil {
		t.Fatal(err)
	}
	if fourth.PrevHash != third.Hash {
		t.Errorf("legal follow-up append did not continue from the third event")
	}
	if reopened.Len() != 4 {
		t.Errorf("reopened chain lost an event: len=%d", reopened.Len())
	}
	if err := reopened.Verify(); err != nil {
		t.Errorf("legal follow-up could not recover the corrupted chain: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	afterSecondRestart, restartErr := audit.Open(path)
	if restartErr != nil {
		t.Errorf("second restart could not load the four-event chain: %v", restartErr)
	} else {
		defer afterSecondRestart.Close()
		if afterSecondRestart.Len() != 4 || afterSecondRestart.LastHash() != fourth.Hash {
			t.Errorf("second restart restored the wrong chain tail: len=%d tail=%s", afterSecondRestart.Len(), afterSecondRestart.LastHash())
		}
		if err := afterSecondRestart.Verify(); err != nil {
			t.Errorf("second restart chain verification failed: %v", err)
		}
	}
}
