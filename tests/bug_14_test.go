package tests

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/repository"
)

type bug14Persister struct {
	data map[string]any
	fail bool
}

func (p *bug14Persister) Load() (map[string]any, error) {
	return cloneBug14Data(p.data), nil
}

func (p *bug14Persister) Save(data map[string]any) error {
	if p.fail {
		return repository.ErrStoreClosed
	}
	p.data = cloneBug14Data(data)
	return nil
}

func cloneBug14Data(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func bug14Encoded(data map[string]any) string {
	b, _ := json.Marshal(data)
	return string(b)
}

func TestBug14CommitRollbackOnPersistFailure(t *testing.T) {
	ctx := context.Background()
	store := &bug14Persister{}
	service := application.NewService(store)
	pre, err := service.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "c", NamespaceID: "n", Key: "rollback", Digest: "digest",
	})
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := service.Occupy(ctx, application.OccupyRequest{
		RequestID: pre.RequestID, ExpectedVersion: pre.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	durableBefore := bug14Encoded(store.data)

	store.fail = true
	_, err = service.Commit(ctx, application.CommitRequest{
		RequestID:       pre.RequestID,
		CredentialID:    occupied.CredentialID,
		Generation:      occupied.Generation,
		ExpectedVersion: occupied.Version,
		Payload:         map[string]any{"state": "durable"},
	})
	if err == nil || !repository.IsDurableSaveFailure(err) {
		t.Errorf("commit did not report the durable save failure: %v", err)
	}

	afterFailure, findErr := service.FindRequest(ctx, pre.RequestID)
	if findErr != nil {
		t.Fatal(findErr)
	}
	if afterFailure.Status != application.StatusOccupied || afterFailure.Version != occupied.Version || afterFailure.ResultID != "" {
		t.Errorf("failed commit changed in-memory request: %#v", afterFailure)
	}
	if durableAfter := bug14Encoded(store.data); durableAfter != durableBefore {
		t.Errorf("failed save changed durable snapshot\nbefore=%s\nafter=%s", durableBefore, durableAfter)
	}
	if replay, replayErr := service.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID}); !errors.Is(replayErr, application.ErrIllegalTransition) {
		t.Errorf("failed commit left a replayable result: response=%#v err=%v", replay, replayErr)
	}

	store.fail = false
	if _, retryErr := service.Commit(ctx, application.CommitRequest{
		RequestID:       pre.RequestID,
		CredentialID:    occupied.CredentialID,
		Generation:      occupied.Generation,
		ExpectedVersion: occupied.Version,
		Payload:         map[string]any{"state": "durable"},
	}); retryErr != nil {
		t.Errorf("same-instance legal retry failed after storage recovered: %v", retryErr)
	}

	reopened := application.NewService(store)
	recovered, err := reopened.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != application.StatusOccupied || recovered.Version != occupied.Version || recovered.ResultID != "" {
		t.Errorf("reopen did not retain the pre-commit request: %#v", recovered)
	}
	committed, err := reopened.Commit(ctx, application.CommitRequest{
		RequestID:       pre.RequestID,
		CredentialID:    occupied.CredentialID,
		Generation:      occupied.Generation,
		ExpectedVersion: occupied.Version,
		Payload:         map[string]any{"state": "durable"},
	})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := reopened.Replay(ctx, application.ReplayRequest{RequestID: pre.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if committed.ResultID == "" || replay.ResultID != committed.ResultID || replay.Payload["state"] != "durable" {
		t.Errorf("recovered legal commit did not produce the expected result: commit=%#v replay=%#v", committed, replay)
	}

	// The file repository must roll its in-memory item back when the atomic write fails.
	repoDir := t.TempDir()
	fileStore, err := repository.NewFileStore(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	initialItem, err := fileStore.Put(ctx, repository.Item{
		ID: "request-state", Kind: "commit_state", Data: map[string]any{"status": "occupied"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repoDir, "repository.json.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, writeErr := fileStore.Put(ctx, repository.Item{
		ID: "request-state", Kind: "commit_state", Version: initialItem.Version + 1,
		Data: map[string]any{"status": "committed"},
	})
	if writeErr == nil {
		t.Errorf("repository update unexpectedly succeeded after its atomic-write path was blocked")
	}
	inMemoryItem, err := fileStore.Get(ctx, "commit_state", "request-state")
	if err != nil {
		t.Fatal(err)
	}
	if got := inMemoryItem.Data["status"]; got != "occupied" {
		t.Errorf("repository kept the failed write in memory: got status %v", got)
	}
	reopenedStore, err := repository.NewFileStore(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	durableItem, err := reopenedStore.Get(ctx, "commit_state", "request-state")
	if err != nil {
		t.Fatal(err)
	}
	if got := durableItem.Data["status"]; got != "occupied" {
		t.Errorf("failed atomic write changed the reopened repository: got status %v", got)
	}
}
