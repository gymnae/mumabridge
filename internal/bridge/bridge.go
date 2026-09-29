package bridge

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"time"
)

// Snapshot is an authoritative participant view from one transport generation.
type Snapshot struct {
	Generation uint64
	Side       Side
	Members    []Participant
	Connected  bool
}

// Source owns a reconnecting transport connection and publishes authoritative
// snapshots. Implementations must close Updates when Run returns.
type Source interface {
	Run(context.Context, chan<- Snapshot) error
}

// Service serializes transport updates and owns reconciliation state.
type Service struct {
	log        *slog.Logger
	reconciler *Reconciler
	sources    []Source
	ready      atomic.Bool
}

func New(log *slog.Logger, reconciler *Reconciler, sources ...Source) *Service {
	return &Service{log: log, reconciler: reconciler, sources: sources}
}

func (s *Service) Run(ctx context.Context) error {
	if s.reconciler == nil {
		return errors.New("bridge reconciler is required")
	}
	updates := make(chan Snapshot, 32)
	errs := make(chan error, len(s.sources))
	for _, source := range s.sources {
		source := source
		go func() { errs <- source.Run(ctx, updates) }()
	}
	members := map[Side]map[string]Participant{SideMatrix: {}, SideMumble: {}}
	generations := map[Side]uint64{}
	connected := map[Side]bool{}
	defer func() {
		s.ready.Store(false)
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.reconciler.Close(shutdown); err != nil {
			s.log.Error("close bridge ghosts", "error", err)
		}
	}()
	s.log.Info("bridge supervisor started")
	for {
		select {
		case <-ctx.Done():
			s.log.Info("bridge supervisor stopping")
			return nil
		case err := <-errs:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		case update := <-updates:
			if update.Generation < generations[update.Side] {
				continue
			}
			generations[update.Side] = update.Generation
			connected[update.Side] = update.Connected
			next := make(map[string]Participant, len(update.Members))
			for _, participant := range update.Members {
				if participant.ID != "" {
					next[participant.ID] = participant
				}
			}
			members[update.Side] = next
			all := make([]Participant, 0, len(members[SideMatrix])+len(members[SideMumble]))
			for _, side := range []Side{SideMatrix, SideMumble} {
				for _, participant := range members[side] {
					all = append(all, participant)
				}
			}
			if err := s.reconciler.Reconcile(ctx, all); err != nil {
				s.log.Error("reconcile participants", "error", err)
			}
			s.ready.Store(connected[SideMatrix] && connected[SideMumble])
		}
	}
}

func (s *Service) Ready() bool { return s.ready.Load() }

// RetryDelay returns capped exponential backoff with up to 25 percent jitter.
func RetryDelay(attempt int, base, maximum time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := base
	for i := 0; i < attempt && delay < maximum/2; i++ {
		delay *= 2
	}
	if delay > maximum {
		delay = maximum
	}
	jitterLimit := int64(delay / 4)
	if jitterLimit < 1 {
		jitterLimit = 1
	}
	return delay + time.Duration(rand.Int63n(jitterLimit)) // #nosec G404 -- jitter is not security-sensitive.
}
