package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/domain"
)

func TestBug03LateCommitAfterTakeover(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	service := application.NewService(func() time.Time { return now })
	registered, err := service.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "worker-a", NamespaceID: "execution", Key: "late-commit-03", Digest: "digest-03",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Occupy(ctx, application.OccupyRequest{
		RequestID: registered.RequestID, ExpectedVersion: registered.Version, LeaseTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	taken, err := service.TakeoverExpired(ctx, application.TakeoverRequest{
		RequestID: registered.RequestID, ExpectedVersion: first.Version, LeaseTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeLate, err := service.FindRequest(ctx, registered.RequestID)
	if err != nil {
		t.Fatal(err)
	}

	_, lateBeforeErr := service.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: first.CredentialID,
		Generation: first.Generation, ExpectedVersion: taken.Version,
		Payload: map[string]any{"source": "old-before-current-commit"},
	})
	if !errors.Is(lateBeforeErr, domain.ErrLateCommit) {
		t.Errorf("old generation before current commit returned %v, want domain.ErrLateCommit", lateBeforeErr)
	}
	afterLateBefore, err := service.FindRequest(ctx, registered.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if afterLateBefore.Version != beforeLate.Version || afterLateBefore.Generation != beforeLate.Generation ||
		afterLateBefore.CredentialID != beforeLate.CredentialID || afterLateBefore.Status != application.StatusOccupied {
		t.Errorf("rejected old generation changed takeover state: before=%#v after=%#v", beforeLate, afterLateBefore)
	}

	currentCommit, err := service.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: taken.CredentialID,
		Generation: taken.Generation, ExpectedVersion: taken.Version,
		Payload: map[string]any{"source": "new"},
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeSecondLate, err := service.FindRequest(ctx, registered.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	_, lateAfterErr := service.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: first.CredentialID,
		Generation: first.Generation, ExpectedVersion: currentCommit.Version,
		Payload: map[string]any{"source": "old-after-current-commit"},
	})
	if !errors.Is(lateAfterErr, domain.ErrLateCommit) {
		t.Errorf("old generation after current commit returned %v, want domain.ErrLateCommit", lateAfterErr)
	}
	afterSecondLate, err := service.FindRequest(ctx, registered.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if afterSecondLate.Version != beforeSecondLate.Version || afterSecondLate.ResultID != beforeSecondLate.ResultID ||
		afterSecondLate.Generation != beforeSecondLate.Generation || afterSecondLate.Status != application.StatusCommitted {
		t.Errorf("late retry changed committed generation: before=%#v after=%#v", beforeSecondLate, afterSecondLate)
	}

	idempotent, err := service.Commit(ctx, application.CommitRequest{
		RequestID: registered.RequestID, CredentialID: taken.CredentialID,
		Generation: taken.Generation, ExpectedVersion: afterSecondLate.Version,
		Payload: map[string]any{"source": "ignored-same-generation-retry"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !idempotent.Hit || idempotent.ResultID != currentCommit.ResultID {
		t.Errorf("same-generation retry was not idempotent: %#v", idempotent)
	}
	replayed, err := service.Replay(ctx, application.ReplayRequest{RequestID: registered.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ResultID != currentCommit.ResultID || replayed.Generation != taken.Generation || replayed.Payload["source"] != "new" {
		t.Errorf("replay was contaminated by an old or duplicate commit: %#v", replayed)
	}
}
