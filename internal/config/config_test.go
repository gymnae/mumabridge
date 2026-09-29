package config

import "testing"

func TestValidateRejectsMissingRequiredValues(t *testing.T) {
	cfg := Config{Media: Media{SampleRate: 48000, Channels: 1, FrameDuration: 20, QueueFrames: 1}, Limits: Limits{Participants: 1}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing configuration error")
	}
}

func TestValidateAcceptsMVPConfiguration(t *testing.T) {
	cfg := Config{
		Matrix:  Matrix{HomeserverURL: "https://matrix.example", ServerName: "example", RoomID: "!room:example", ASToken: "as", HSToken: "hs"},
		LiveKit: LiveKit{URL: "wss://livekit.example", APIKey: "key", APISecret: "secret", Room: "room"},
		Mumble:  Mumble{Address: "mumble.example:64738"},
		Media:   Media{SampleRate: 48000, Channels: 1, FrameDuration: 20, QueueFrames: 1},
		Limits:  Limits{Participants: 1},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
