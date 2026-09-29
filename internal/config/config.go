package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains all process configuration. Secrets are deliberately kept as
// strings and Config must never be logged as a whole.
type Config struct {
	HTTP      HTTP
	Matrix    Matrix
	LiveKit   LiveKit
	Mumble    Mumble
	Media     Media
	Limits    Limits
	LogLevel  string
	StatePath string
}

type HTTP struct {
	Address string
}

type Matrix struct {
	HomeserverURL   string
	ServerName      string
	RoomID          string
	ASToken         string
	HSToken         string
	SenderLocalpart string
	GhostPrefix     string
}

type LiveKit struct {
	URL       string
	APIKey    string
	APISecret string
	Room      string
}

type Mumble struct {
	Address     string
	Channel     string
	Username    string
	Password    string
	Certificate string
	Key         string
	InsecureTLS bool
	GhostPrefix string
}

type Media struct {
	SampleRate    int
	Channels      int
	FrameDuration time.Duration
	QueueFrames   int
}

type Limits struct {
	Participants int
}

func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTP{Address: env("HTTP_ADDRESS", ":8080")},
		Matrix: Matrix{
			HomeserverURL:   env("MATRIX_HOMESERVER_URL", ""),
			ServerName:      env("MATRIX_SERVER_NAME", ""),
			RoomID:          env("MATRIX_ROOM_ID", ""),
			ASToken:         env("MATRIX_AS_TOKEN", ""),
			HSToken:         env("MATRIX_HS_TOKEN", ""),
			SenderLocalpart: env("MATRIX_SENDER_LOCALPART", "mumabridge"),
			GhostPrefix:     env("MATRIX_GHOST_PREFIX", "mumabridge_mumble_"),
		},
		LiveKit: LiveKit{
			URL:       env("LIVEKIT_URL", ""),
			APIKey:    env("LIVEKIT_API_KEY", ""),
			APISecret: env("LIVEKIT_API_SECRET", ""),
			Room:      env("LIVEKIT_ROOM", env("MATRIX_ROOM_ID", "")),
		},
		Mumble: Mumble{
			Address:     env("MUMBLE_ADDRESS", ""),
			Channel:     env("MUMBLE_CHANNEL", "Root"),
			Username:    env("MUMBLE_USERNAME", "mumabridge"),
			Password:    env("MUMBLE_PASSWORD", ""),
			Certificate: env("MUMBLE_CERT_FILE", ""),
			Key:         env("MUMBLE_KEY_FILE", ""),
			InsecureTLS: envBool("MUMBLE_INSECURE_TLS", false),
			GhostPrefix: env("MUMBLE_GHOST_PREFIX", "[mx] "),
		},
		Media: Media{
			SampleRate:    envInt("MEDIA_SAMPLE_RATE", 48000),
			Channels:      envInt("MEDIA_CHANNELS", 1),
			FrameDuration: time.Duration(envInt("MEDIA_FRAME_MS", 20)) * time.Millisecond,
			QueueFrames:   envInt("MEDIA_QUEUE_FRAMES", 25),
		},
		Limits:    Limits{Participants: envInt("MAX_PARTICIPANTS", 32)},
		LogLevel:  env("LOG_LEVEL", "info"),
		StatePath: env("STATE_PATH", "/var/lib/mumabridge"),
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	var missing []string
	for key, value := range map[string]string{
		"MATRIX_HOMESERVER_URL": c.Matrix.HomeserverURL,
		"MATRIX_SERVER_NAME":    c.Matrix.ServerName,
		"MATRIX_ROOM_ID":        c.Matrix.RoomID,
		"MATRIX_AS_TOKEN":       c.Matrix.ASToken,
		"MATRIX_HS_TOKEN":       c.Matrix.HSToken,
		"LIVEKIT_URL":           c.LiveKit.URL,
		"LIVEKIT_API_KEY":       c.LiveKit.APIKey,
		"LIVEKIT_API_SECRET":    c.LiveKit.APISecret,
		"LIVEKIT_ROOM":          c.LiveKit.Room,
		"MUMBLE_ADDRESS":        c.Mumble.Address,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if c.Media.SampleRate != 48000 || c.Media.Channels != 1 {
		return errors.New("MVP requires 48000 Hz mono media")
	}
	if c.Media.FrameDuration <= 0 || c.Media.QueueFrames < 1 || c.Limits.Participants < 1 {
		return errors.New("media frame duration, queue size, and participant limit must be positive")
	}
	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
