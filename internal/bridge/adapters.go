package bridge

import (
	"context"
	"time"

	"github.com/gymnae/mumabridge/internal/livekit"
	"github.com/gymnae/mumabridge/internal/matrix"
	"github.com/gymnae/mumabridge/internal/media"
	"github.com/gymnae/mumabridge/internal/mumble"
)

// GhostFactoryAdapter composes the concrete transports used for opposite-side
// ghost creation. Matrix ghosts also publish their Mumble user's audio through
// an independently identified LiveKit participant.
type GhostFactoryAdapter struct {
	Matrix      *matrix.Client
	LiveKit     *livekit.Manager
	Mumble      *mumble.Manager
	RoomID      string
	GhostPrefix string
}

func (f *GhostFactoryAdapter) CreateMatrixGhost(ctx context.Context, participant Participant) (Ghost, error) {
	localpart := matrix.GhostLocalpart(f.GhostPrefix, participant.ID, participant.DisplayName)
	if err := f.Matrix.EnsureGhost(ctx, localpart, participant.DisplayName, f.RoomID); err != nil {
		return nil, err
	}
	identity := f.Matrix.UserID(localpart)
	if err := f.LiveKit.AddPublishingGhost(ctx, identity, participant.DisplayName, participant.ID, audioOrSilence(participant.Audio)); err != nil {
		_ = f.Matrix.LeaveGhost(context.Background(), identity, f.RoomID)
		return nil, err
	}
	return ghostFunc(func(closeCtx context.Context) error {
		first := f.LiveKit.RemoveGhost(closeCtx, identity)
		second := f.Matrix.LeaveGhost(closeCtx, identity, f.RoomID)
		if first != nil {
			return first
		}
		return second
	}), nil
}

func (f *GhostFactoryAdapter) CreateMumbleGhost(ctx context.Context, participant Participant) (Ghost, error) {
	if err := f.Mumble.AddGhost(ctx, participant.ID, participant.DisplayName, audioOrSilence(participant.Audio)); err != nil {
		return nil, err
	}
	return ghostFunc(func(closeCtx context.Context) error { return f.Mumble.RemoveGhost(closeCtx, participant.ID) }), nil
}

type ghostFunc func(context.Context) error

func (f ghostFunc) Close(ctx context.Context) error { return f(ctx) }

func audioOrSilence(frames <-chan media.Frame) <-chan media.Frame {
	if frames != nil {
		return frames
	}
	return make(chan media.Frame)
}

// MumbleSource maintains the observer connection and translates complete user
// snapshots into supervisor snapshots. A new generation invalidates callbacks
// from previous connections.
type MumbleSource struct {
	Dialer      mumble.Dialer
	Address     string
	Username    string
	Password    string
	Channel     string
	GhostPrefix string
	InsecureTLS bool
}

func (s *MumbleSource) Run(ctx context.Context, updates chan<- Snapshot) error {
	var generation uint64
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return nil
		}
		generation++
		session, err := s.Dialer.Connect(ctx, s.Address, s.Username, s.Password, s.Channel, s.InsecureTLS)
		if err != nil {
			if !sleepContext(ctx, RetryDelay(attempt, reconnectBase, reconnectMaximum)) {
				return nil
			}
			continue
		}
		attempt = 0
		for {
			select {
			case <-ctx.Done():
				_ = session.Close(context.Background())
				return nil
			case users, ok := <-session.Users():
				if !ok {
					_ = session.Close(context.Background())
					updates <- Snapshot{Generation: generation, Side: SideMumble}
					goto reconnect
				}
				members := make([]Participant, 0, len(users))
				for _, user := range users {
					members = append(members, Participant{ID: user.ID, DisplayName: user.Name, Side: SideMumble, BridgeOwned: user.BridgeOwned || mumble.IsBridgeOwned(user.Name, s.GhostPrefix), Audio: user.Audio})
				}
				updates <- Snapshot{Generation: generation, Side: SideMumble, Members: members, Connected: true}
			}
		}
	reconnect:
		if !sleepContext(ctx, RetryDelay(attempt, reconnectBase, reconnectMaximum)) {
			return nil
		}
	}
}

const (
	reconnectBase    = 500 * time.Millisecond
	reconnectMaximum = 30 * time.Second
)

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
