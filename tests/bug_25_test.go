package tests

import (
	"context"
	"math"
	"testing"
	"time"

	"hwj-macgo-0018/application"
	"hwj-macgo-0018/query"
)

func commitForRate(t *testing.T, app *application.UseCase, key string) application.CommitResponse {
	t.Helper()
	ctx := context.Background()
	pre, err := app.PreRegister(ctx, application.PreRegisterRequest{CallerID: "rate", NamespaceID: "replay", Key: key, Digest: "digest-" + key, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	occupied, err := app.Occupy(ctx, application.OccupyRequest{RequestID: pre.RequestID, ExpectedVersion: pre.Version, LeaseTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	committed, err := app.Commit(ctx, application.CommitRequest{RequestID: pre.RequestID, CredentialID: occupied.CredentialID, Generation: occupied.Generation, ExpectedVersion: occupied.Version, Payload: map[string]any{"key": key}})
	if err != nil {
		t.Fatal(err)
	}
	return committed
}

func TestBug25ReplayHitRateDenominator(t *testing.T) {
	ctx := context.Background()
	app := application.NewUseCase()
	first := commitForRate(t, app, "first")
	commitForRate(t, app, "second")
	for i := 0; i < 3; i++ {
		if _, err := app.Replay(ctx, application.ReplayRequest{RequestID: first.RequestID}); err != nil {
			t.Fatal(err)
		}
	}
	rate, err := app.ReplayHitRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(rate.Rate-0.6) > 1e-9 {
		t.Fatalf("diagnosis expected application rate 0.6 from three replay events over five events, got %.6f", rate.Rate)
	}

	derived := query.NewService([]query.Row{
		{ID: "first", Fields: map[string]any{"status": "committed", "replay_hit": true}},
		{ID: "second", Fields: map[string]any{"status": "committed", "replay_hit": false}},
	}).ReplayHitRate()
	if math.Abs(derived-(1.0/3.0)) > 1e-9 {
		t.Fatalf("diagnosis expected derived rate 0.333333 from the expanded denominator, got %.6f", derived)
	}
}
