package bridge

import (
	"context"
	"testing"
)

type fakeGhost struct{ closed bool }

func (g *fakeGhost) Close(context.Context) error { g.closed = true; return nil }

type fakeFactory struct {
	matrix map[string]*fakeGhost
	mumble map[string]*fakeGhost
}

func newFakeFactory() *fakeFactory {
	return &fakeFactory{matrix: make(map[string]*fakeGhost), mumble: make(map[string]*fakeGhost)}
}

func (f *fakeFactory) CreateMatrixGhost(_ context.Context, p Participant) (Ghost, error) {
	ghost := &fakeGhost{}
	f.matrix[p.ID] = ghost
	return ghost, nil
}

func (f *fakeFactory) CreateMumbleGhost(_ context.Context, p Participant) (Ghost, error) {
	ghost := &fakeGhost{}
	f.mumble[p.ID] = ghost
	return ghost, nil
}

func TestReconcilerCreatesOppositeSideGhostsAndFiltersOwned(t *testing.T) {
	factory := newFakeFactory()
	reconciler := NewReconciler(factory, 10)
	err := reconciler.Reconcile(context.Background(), []Participant{
		{ID: "m1", Side: SideMumble},
		{ID: "mx1", Side: SideMatrix},
		{ID: "owned", Side: SideMatrix, BridgeOwned: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(factory.matrix) != 1 || factory.matrix["m1"] == nil {
		t.Fatalf("unexpected Matrix ghosts: %#v", factory.matrix)
	}
	if len(factory.mumble) != 1 || factory.mumble["mx1"] == nil {
		t.Fatalf("unexpected Mumble ghosts: %#v", factory.mumble)
	}
}

func TestReconcilerClosesDepartedGhost(t *testing.T) {
	factory := newFakeFactory()
	reconciler := NewReconciler(factory, 10)
	_ = reconciler.Reconcile(context.Background(), []Participant{{ID: "m1", Side: SideMumble}})
	ghost := factory.matrix["m1"]
	if err := reconciler.Reconcile(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !ghost.closed {
		t.Fatal("departed ghost was not closed")
	}
}
