package bridge

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type sourceFunc func(context.Context, chan<- Snapshot) error

func (f sourceFunc) Run(ctx context.Context, updates chan<- Snapshot) error { return f(ctx, updates) }

func TestServiceReconcilesSnapshotsAndIgnoresStaleGeneration(t *testing.T) {
	factory := newFakeFactory()
	reconciler := NewReconciler(factory, 10)
	ctx, cancel := context.WithCancel(context.Background())
	source := sourceFunc(func(ctx context.Context, updates chan<- Snapshot) error {
		updates <- Snapshot{Generation: 2, Side: SideMumble, Connected: true, Members: []Participant{{ID: "new", Side: SideMumble}}}
		updates <- Snapshot{Generation: 1, Side: SideMumble, Connected: true, Members: []Participant{{ID: "stale", Side: SideMumble}}}
		updates <- Snapshot{Generation: 1, Side: SideMatrix, Connected: true}
		<-ctx.Done()
		return nil
	})
	service := New(slog.New(slog.NewTextHandler(io.Discard, nil)), reconciler, source)
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	deadline := time.After(time.Second)
	for !service.Ready() {
		select {
		case <-deadline:
			t.Fatal("service did not become ready")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if factory.matrix["new"] == nil || factory.matrix["stale"] != nil {
		t.Fatalf("stale generation changed ghosts: %#v", factory.matrix)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if service.Ready() {
		t.Fatal("service remained ready after shutdown")
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		delay := RetryDelay(attempt, time.Second, 10*time.Second)
		if delay < time.Second || delay >= 12500*time.Millisecond {
			t.Fatalf("unexpected retry delay %s", delay)
		}
	}
}
