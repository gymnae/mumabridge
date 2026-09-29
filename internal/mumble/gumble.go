package mumble

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"
	"sync"
	"time"

	"layeh.com/gumble/gumble"

	"github.com/gymnae/mumabridge/internal/media"
)

type GumbleDialer struct{}

func NewGumbleDialer() *GumbleDialer { return &GumbleDialer{} }

func (d *GumbleDialer) Connect(ctx context.Context, address, username, password, channel string, insecureTLS bool) (Session, error) {
	config := gumble.NewConfig()
	config.Username = username
	config.Password = password
	config.AudioInterval = 20 * time.Millisecond
	session := &gumbleSession{users: make(chan []User, 16), closed: make(chan struct{})}
	config.AttachAudio(session)
	config.Attach(session)
	client, err := gumble.DialWithDialer(nil, address, config, &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecureTLS}) // #nosec G402 -- explicitly configured opt-out for private Mumble deployments.
	if err != nil {
		return nil, err
	}
	session.client = client
	if target := client.Channels.Find(splitChannel(channel)...); target != nil && client.Self.Channel != target {
		client.Self.Move(target)
	}
	session.emitUsers()
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close(context.Background())
		case <-session.closed:
		}
	}()
	return session, nil
}

type gumbleSession struct {
	client    *gumble.Client
	users     chan []User
	closed    chan struct{}
	closeOnce sync.Once
}

func (s *gumbleSession) SendPCM(ctx context.Context, frames <-chan media.Frame) error {
	outgoing := s.client.AudioOutgoing()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.closed:
			return nil
		case frame, ok := <-frames:
			if !ok {
				return nil
			}
			buffer := append(gumble.AudioBuffer(nil), frame...)
			select {
			case outgoing <- buffer:
			case <-ctx.Done():
				return ctx.Err()
			case <-s.closed:
				return nil
			}
		}
	}
}

func (s *gumbleSession) Users() <-chan []User { return s.users }

func (s *gumbleSession) Close(context.Context) error {
	var disconnectErr error
	s.closeOnce.Do(func() {
		close(s.closed)
		close(s.users)
		if s.client != nil {
			disconnectErr = s.client.Disconnect()
		}
	})
	return disconnectErr
}

func (s *gumbleSession) OnAudioStream(event *gumble.AudioStreamEvent) {
	queue := media.NewQueue(25)
	user := User{ID: userID(event.User), Name: event.User.Name, Audio: queue.Frames()}
	select {
	case s.users <- []User{user}:
	case <-s.closed:
		return
	}
	go func() {
		for packet := range event.C {
			queue.Push(media.Frame(packet.AudioBuffer))
		}
	}()
}

func (s *gumbleSession) OnConnect(*gumble.ConnectEvent) {}

func (s *gumbleSession) OnTextMessage(*gumble.TextMessageEvent) {}

func (s *gumbleSession) OnUserChange(*gumble.UserChangeEvent) { s.emitUsers() }

func (s *gumbleSession) OnUserRemove(*gumble.UserRemoveEvent) { s.emitUsers() }

func (s *gumbleSession) OnChannelChange(*gumble.ChannelChangeEvent) { s.emitUsers() }

func (s *gumbleSession) OnChannelRemove(*gumble.ChannelRemoveEvent) { s.emitUsers() }

func (s *gumbleSession) OnPermissionDenied(*gumble.PermissionDeniedEvent) {}

func (s *gumbleSession) OnDisconnect(*gumble.DisconnectEvent) {
	s.closeOnce.Do(func() {
		close(s.closed)
		close(s.users)
	})
}

func (s *gumbleSession) emitUsers() {
	users := make([]User, 0, len(s.client.Users))
	for _, user := range s.client.Users {
		if user == s.client.Self {
			continue
		}
		users = append(users, User{ID: userID(user), Name: user.Name})
	}
	select {
	case s.users <- users:
	default:
	}
}

func userID(user *gumble.User) string {
	if user == nil {
		return ""
	}
	if user.UserID != 0 {
		return "registered:" + strconv.FormatUint(uint64(user.UserID), 10)
	}
	return "session:" + strconv.FormatUint(uint64(user.Session), 10)
}

func splitChannel(channel string) []string {
	channel = strings.Trim(channel, "/")
	if channel == "" || channel == "Root" {
		return nil
	}
	return strings.Split(channel, "/")
}

var (
	_ gumble.AudioListener      = (*gumbleSession)(nil)
	_ gumble.EventListener      = (*gumbleSession)(nil)
)
