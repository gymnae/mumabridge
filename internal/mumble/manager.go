package mumble

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/gymnae/mumabridge/internal/media"
)

type User struct {
	ID          string
	Name        string
	BridgeOwned bool
	Audio       <-chan media.Frame
}

type Session interface {
	SendPCM(context.Context, <-chan media.Frame) error
	Users() <-chan []User
	Close(context.Context) error
}

type Dialer interface {
	Connect(context.Context, string, string, string, string, bool) (Session, error)
}

type Config struct {
	Address     string
	Channel     string
	Password    string
	GhostPrefix string
	InsecureTLS bool
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

func (m *Manager) AddGhost(ctx context.Context, sourceID, displayName string, frames <-chan media.Frame) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[sourceID]; exists {
		return nil
	}
	username := GhostName(m.config.GhostPrefix, displayName, sourceID)
	session, err := m.dialer.Connect(ctx, m.config.Address, username, m.config.Password, m.config.Channel, m.config.InsecureTLS)
	if err != nil {
		return fmt.Errorf("connect Mumble ghost %q: %w", username, err)
	}
	m.sessions[sourceID] = session
	go func() {
		if err := session.SendPCM(ctx, frames); err != nil {
			_ = m.RemoveGhost(context.Background(), sourceID)
		}
	}()
	return nil
}

func (m *Manager) RemoveGhost(ctx context.Context, sourceID string) error {
	m.mu.Lock()
	session, exists := m.sessions[sourceID]
	if exists {
		delete(m.sessions, sourceID)
	}
	m.mu.Unlock()
	if !exists {
		return nil
	}
	return session.Close(ctx)
}

func GhostName(prefix, displayName, stableID string) string {
	var clean strings.Builder
	for _, r := range displayName {
		if unicode.IsControl(r) {
			continue
		}
		clean.WriteRune(r)
	}
	name := strings.TrimSpace(clean.String())
	if name == "" {
		name = "Matrix user"
	}
	digest := sha256.Sum256([]byte(stableID))
	suffix := "~" + hex.EncodeToString(digest[:3])
	maximum := 64 - len(prefix) - len(suffix)
	if maximum < 1 {
		maximum = 1
	}
	nameRunes := []rune(name)
	if len(nameRunes) > maximum {
		name = string(nameRunes[:maximum])
	}
	return prefix + name + suffix
}

func IsBridgeOwned(name, prefix string) bool { return strings.HasPrefix(name, prefix) }
