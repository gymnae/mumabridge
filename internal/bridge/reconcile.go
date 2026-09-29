package bridge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/gymnae/mumabridge/internal/media"
)

type Side string

const (
	SideMatrix Side = "matrix"
	SideMumble Side = "mumble"
)

type Participant struct {
	ID          string
	DisplayName string
	Side        Side
	BridgeOwned bool
	Audio       <-chan media.Frame
}

type Ghost interface {
	Close(context.Context) error
}

type GhostFactory interface {
	CreateMatrixGhost(context.Context, Participant) (Ghost, error)
	CreateMumbleGhost(context.Context, Participant) (Ghost, error)
}

type Reconciler struct {
	mu      sync.Mutex
	factory GhostFactory
	limit   int
	matrix  map[string]Ghost
	mumble  map[string]Ghost
}

func NewReconciler(factory GhostFactory, limit int) *Reconciler {
	return &Reconciler{factory: factory, limit: limit, matrix: make(map[string]Ghost), mumble: make(map[string]Ghost)}
}

func (r *Reconciler) Reconcile(ctx context.Context, participants []Participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	desiredMatrix := make(map[string]Participant)
	desiredMumble := make(map[string]Participant)
	for _, participant := range participants {
		if participant.BridgeOwned || participant.ID == "" {
			continue
		}
		switch participant.Side {
		case SideMumble:
			desiredMatrix[participant.ID] = participant
		case SideMatrix:
			desiredMumble[participant.ID] = participant
		}
	}
	if len(desiredMatrix)+len(desiredMumble) > r.limit {
		return fmt.Errorf("participant limit exceeded: %d > %d", len(desiredMatrix)+len(desiredMumble), r.limit)
	}
	var errs []error
	errs = append(errs, r.removeStale(ctx, r.matrix, desiredMatrix)...)
	errs = append(errs, r.removeStale(ctx, r.mumble, desiredMumble)...)
	for _, id := range sortedMissing(r.matrix, desiredMatrix) {
		ghost, err := r.factory.CreateMatrixGhost(ctx, desiredMatrix[id])
		if err != nil {
			errs = append(errs, fmt.Errorf("create Matrix ghost for %s: %w", id, err))
			continue
		}
		r.matrix[id] = ghost
	}
	for _, id := range sortedMissing(r.mumble, desiredMumble) {
		ghost, err := r.factory.CreateMumbleGhost(ctx, desiredMumble[id])
		if err != nil {
			errs = append(errs, fmt.Errorf("create Mumble ghost for %s: %w", id, err))
			continue
		}
		r.mumble[id] = ghost
	}
	return errors.Join(errs...)
}

// Close removes all managed ghosts. It is safe to call repeatedly and is used
// by the supervisor to guarantee deterministic shutdown.
func (r *Reconciler) Close(ctx context.Context) error {
	return r.Reconcile(ctx, nil)
}

func (r *Reconciler) removeStale(ctx context.Context, actual map[string]Ghost, desired map[string]Participant) []error {
	var errs []error
	for id, ghost := range actual {
		if _, keep := desired[id]; keep {
			continue
		}
		if err := ghost.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close ghost for %s: %w", id, err))
			continue
		}
		delete(actual, id)
	}
	return errs
}

func sortedMissing(actual map[string]Ghost, desired map[string]Participant) []string {
	missing := make([]string, 0)
	for id := range desired {
		if _, exists := actual[id]; !exists {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}
