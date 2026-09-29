package livekit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gymnae/mumabridge/internal/media"
)

const BridgeMetadataKey = "mumabridge"

type Participant struct {
	Identity string
	Name     string
	Metadata string
}

type RemoteAudio struct {
	Participant Participant
	Frames      <-chan media.Frame
}

// Session abstracts the LiveKit WebRTC SDK. The transport implementation is
// intentionally isolated because LiveKit's callbacks must not own bridge
// reconciliation state.
type Session interface {
	PublishPCM(context.Context, <-chan media.Frame) error
	RemoteAudio() <-chan RemoteAudio
	Close(context.Context) error
}

type Dialer interface {
	Join(context.Context, string, string) (Session, error)
}

type Config struct {
	URL       string
	APIKey    string
	APISecret string
	Room      string
}

type Manager struct {
	mu       sync.Mutex
	config   Config
	dialer   Dialer
	sessions map[string]Session
}

func NewManager(config Config, dialer Dialer) *Manager {
	return &Manager{config: config, dialer: dialer, sessions: make(map[string]Session)}
}

func (m *Manager) AddPublishingGhost(ctx context.Context, identity, name, sourceID string, frames <-chan media.Frame) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[identity]; exists {
		return nil
	}
	metadata, _ := json.Marshal(map[string]string{BridgeMetadataKey: "true", "source": "mumble", "source_id": sourceID})
	token, err := ParticipantToken(m.config.APIKey, m.config.APISecret, m.config.Room, identity, name, string(metadata), time.Now())
	if err != nil {
		return err
	}
	session, err := m.dialer.Join(ctx, m.config.URL, token)
	if err != nil {
		return fmt.Errorf("join LiveKit room: %w", err)
	}
	m.sessions[identity] = session
	go func() {
		if err := session.PublishPCM(ctx, frames); err != nil {
			_ = m.RemoveGhost(context.Background(), identity)
		}
	}()
	return nil
}

func (m *Manager) RemoveGhost(ctx context.Context, identity string) error {
	m.mu.Lock()
	session, exists := m.sessions[identity]
	if exists {
		delete(m.sessions, identity)
	}
	m.mu.Unlock()
	if !exists {
		return nil
	}
	return session.Close(ctx)
}

func IsBridgeOwned(metadata string) bool {
	var values map[string]any
	if json.Unmarshal([]byte(metadata), &values) != nil {
		return false
	}
	value, _ := values[BridgeMetadataKey].(string)
	return value == "true"
}
