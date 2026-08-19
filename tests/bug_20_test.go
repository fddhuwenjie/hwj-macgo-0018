package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
)

func TestBug20ConcurrentQuerySnapshotDiagnosis(t *testing.T) {
	ctx := context.Background()
	service := application.NewService()
	created, err := service.PreRegister(ctx, application.PreRegisterRequest{
		CallerID: "worker-20", NamespaceID: "render", Key: "scene-20",
		Digest: "digest-20", LeaseTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("pre-register request: %v", err)
	}
	occupied, err := service.Occupy(ctx, application.OccupyRequest{
		RequestID: created.RequestID, ExpectedVersion: created.Version, LeaseTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("occupy request: %v", err)
	}

	queryDone := make(chan application.QueryResponse, 1)
	go func() {
		response, queryErr := service.QueryRequests(ctx, application.QueryRequest{CallerID: "worker-20"})
		if queryErr != nil {
			queryDone <- application.QueryResponse{}
			return
		}
		queryDone <- response
	}()
	time.Sleep(2 * time.Millisecond)
	committed, err := service.Commit(ctx, application.CommitRequest{
		RequestID: created.RequestID, CredentialID: occupied.CredentialID,
		Generation: occupied.Generation, ExpectedVersion: occupied.Version,
		Payload: map[string]any{"artifact": "frame-20"},
	})
	if err != nil {
		t.Fatalf("commit request while query copies snapshot: %v", err)
	}

	response := <-queryDone
	if len(response.Items) != 1 {
		t.Fatalf("query returned %d requests, want 1", len(response.Items))
	}
	item := response.Items[0]
	if item.Status == application.StatusOccupied && item.Version == committed.Version {
		t.Fatalf("query mixed the occupied status with committed version %d", item.Version)
	}
	item.Labels["scope"] = "caller-overwrite"

	again, err := service.QueryRequests(ctx, application.QueryRequest{CallerID: "worker-20"})
	if err != nil {
		t.Fatalf("query after caller mutation: %v", err)
	}
	if got := again.Items[0].Labels["scope"]; got != "worker-20/render/scene-20" {
		t.Fatalf("caller mutation leaked into later query: got %q", got)
	}
	if again.Items[0].Status != application.StatusCommitted || again.Items[0].Version != committed.Version {
		t.Fatalf("later query lost committed state: status=%s version=%d", again.Items[0].Status, again.Items[0].Version)
	}
}
