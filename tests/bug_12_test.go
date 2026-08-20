package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/query"
)

func TestBug12ActiveLeaseWorkbenchPagination(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	service := application.NewService(func() time.Time { return now })
	type lease struct {
		request    application.Request
		credential string
	}
	leases := make([]lease, 0, 3)
	for i, ttl := range []time.Duration{time.Minute, 2 * time.Minute, 3 * time.Minute} {
		pre, err := service.PreRegister(ctx, application.PreRegisterRequest{CallerID: "takeover-desk", NamespaceID: "media-jobs", Key: string(rune('a' + i)), Digest: string(rune('x' + i)), LeaseTTL: ttl})
		if err != nil {
			t.Fatal(err)
		}
		occupied, err := service.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: ttl})
		if err != nil {
			t.Fatal(err)
		}
		request, err := service.FindRequest(ctx, pre.RequestID)
		if err != nil {
			t.Fatal(err)
		}
		leases = append(leases, lease{request: request, credential: occupied.CredentialID})
		now = now.Add(time.Second)
	}

	applicationPage, err := service.QueryRequests(ctx, application.QueryRequest{Status: application.StatusOccupied, Offset: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if applicationPage.Total != 3 || len(applicationPage.Items) != 1 {
		t.Errorf("application page lost the eligible lease total: %#v", applicationPage)
	}

	all, err := service.QueryRequests(ctx, application.QueryRequest{Status: application.StatusOccupied})
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]query.Row, 0, len(all.Items))
	before := make(map[string]application.Request, len(all.Items))
	for _, request := range all.Items {
		before[request.ID] = request
		rows = append(rows, query.Row{ID: request.ID, Fields: map[string]any{
			"status": request.Status, "expires_at": request.LeaseExpiresAt,
		}})
	}
	page := query.NewService(rows).
		Filter(query.HasField("expires_at")).
		Filter(query.FieldEquals("status", application.StatusOccupied)).
		Sort("expires_at", false).
		Page(1, 1)
	if len(page.Items) != 1 || page.Items[0].ID != leases[1].request.ID || page.Total != 3 {
		t.Errorf("stable second lease page is missing or duplicated: %#v", page)
	}

	for id, snapshot := range before {
		after, err := service.FindRequest(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if after.Version != snapshot.Version || after.Status != snapshot.Status || after.CredentialID != snapshot.CredentialID {
			t.Errorf("failed read path mutated lease %s: before=%#v after=%#v", id, snapshot, after)
		}
	}
	selected := leases[1]
	if _, err := service.Commit(ctx, application.CommitRequest{RequestID: selected.request.ID, CredentialID: selected.credential, Generation: selected.request.Generation, ExpectedVersion: selected.request.Version, Payload: map[string]any{"worker": "accepted-after-query"}}); err != nil {
		t.Fatalf("legal commit failed after the bad page read: %v", err)
	}
}
