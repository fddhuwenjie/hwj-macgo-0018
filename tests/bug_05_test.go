package tests

import (
	"context"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/query"
)

func TestBug05ExpirationScanDiagnosis(t *testing.T) {
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	s := application.NewService(func() time.Time { return now })
	ctx := context.Background()

	type activeLease struct {
		requestID    string
		credentialID string
		version      int64
	}
	create := func(key string, ttl time.Duration) activeLease {
		t.Helper()
		pr, err := s.PreRegister(ctx, application.PreRegisterRequest{
			CallerID: "scanner", NamespaceID: "leases", Key: key,
			Digest: "digest-" + key, LeaseTTL: ttl,
		})
		if err != nil {
			t.Fatal(err)
		}
		occupied, err := s.Occupy(ctx, application.OccupyRequest{
			RequestID: pr.RequestID, ExpectedVersion: pr.Version, LeaseTTL: ttl,
		})
		if err != nil {
			t.Fatal(err)
		}
		return activeLease{requestID: pr.RequestID, credentialID: occupied.CredentialID, version: occupied.Version}
	}

	expiring := create("expires-first", time.Minute)
	stillActive := create("expires-later", 5*time.Minute)

	early, err := s.ExpireCredentials(ctx, now.Add(30*time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(early.Expired) != 0 {
		t.Errorf("premature scan expired active credentials: %#v", early.Expired)
	}
	for _, lease := range []activeLease{expiring, stillActive} {
		request, findErr := s.FindRequest(ctx, lease.requestID)
		if findErr != nil {
			t.Errorf("find request after premature scan: %v", findErr)
			continue
		}
		if request.Status != application.StatusOccupied || request.Version != lease.version {
			t.Errorf("premature scan mutated request %s: %#v", lease.requestID, request)
		}
	}

	batch, err := s.ExpireCredentials(ctx, now.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Expired) != 1 {
		t.Errorf("expected exactly one genuinely expired credential: %#v", batch)
	} else {
		item := batch.Expired[0]
		if item.RequestID != expiring.requestID || item.CredentialID != expiring.credentialID || item.Generation != 1 {
			t.Errorf("expiration event lost request/credential linkage: %#v", item)
		}
		if !item.ExpiredAt.Equal(now.Add(time.Minute)) {
			t.Errorf("expiration event has wrong deadline: %s", item.ExpiredAt)
		}
	}
	expiredRequest, err := s.FindRequest(ctx, expiring.requestID)
	if err != nil {
		t.Fatal(err)
	}
	activeRequest, err := s.FindRequest(ctx, stillActive.requestID)
	if err != nil {
		t.Fatal(err)
	}
	if expiredRequest.Status != application.StatusFailed || activeRequest.Status != application.StatusOccupied {
		t.Errorf("scan and request state disagree: expired=%#v active=%#v", expiredRequest, activeRequest)
	}

	repeated, err := s.ExpireCredentials(ctx, now.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(repeated.Expired) != 0 {
		t.Errorf("repeat scan emitted duplicate expiration events: %#v", repeated.Expired)
	}
	afterRepeat, err := s.FindRequest(ctx, stillActive.requestID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRepeat.Version != activeRequest.Version || afterRepeat.Status != application.StatusOccupied {
		t.Errorf("repeat scan mutated non-expired request: before=%#v after=%#v", activeRequest, afterRepeat)
	}

	rows := query.NewService([]query.Row{
		{ID: expiring.requestID, Fields: map[string]any{"status": "executing", "expires_at": now.Add(-time.Second)}},
		{ID: stillActive.requestID, Fields: map[string]any{"status": "executing", "expires_at": now.Add(time.Minute)}},
	})
	hanging := rows.HangingExecutions(now)
	if len(hanging) != 1 || hanging[0].ID != expiring.requestID {
		t.Errorf("expiration query disagrees with scan boundary: %#v", hanging)
	}
}
