package tests

import (
	"context"
	"errors"
	"hwj-macgo-0018/application"
	"hwj-macgo-0018/repository"
	"testing"
)

type bug14Persister struct {
	data map[string]any
	fail bool
}

func (p *bug14Persister) Load() (map[string]any, error) { return p.data, nil }
func (p *bug14Persister) Save(data map[string]any) error {
	if p.fail {
		return repository.ErrStoreClosed
	}
	p.data = data
	return nil
}

func TestBug14CommitRollbackOnPersistFailure(t *testing.T) {
	ctx := context.Background()
	p := &bug14Persister{}
	s := application.NewService(p)
	pre, err := s.PreRegister(ctx, application.PreRegisterRequest{CallerID: "c", NamespaceID: "n", Key: "k", Digest: "d"})
	if err != nil {
		t.Fatal(err)
	}
	occ, err := s.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version})
	if err != nil {
		t.Fatal(err)
	}
	p.fail = true
	_, err = s.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occ.CredentialID, Generation: occ.Generation, ExpectedVersion: occ.Version, Payload: map[string]any{"ok": true}})
	if err == nil || !repository.IsDurableSaveFailure(err) {
		t.Fatalf("commit did not report durable save failure: %v", err)
	}
	got, err := s.FindRequest(ctx, pre.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != application.StatusCommitted || got.Version <= occ.Version {
		t.Fatalf("diagnosis expected failed persistence to leave a half-updated request: %#v", got)
	}
	if !errors.Is(repository.ErrStoreClosed, repository.ErrStoreClosed) {
		t.Fatal("sentinel mismatch")
	}
}
