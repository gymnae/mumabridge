package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type videoGrant struct {
	RoomJoin     bool   `json:"roomJoin"`
	Room         string `json:"room"`
	CanPublish   bool   `json:"canPublish"`
	CanSubscribe bool   `json:"canSubscribe"`
}

type tokenClaims struct {
	Issuer    string     `json:"iss"`
	Subject   string     `json:"sub"`
	Name      string     `json:"name"`
	Metadata  string     `json:"metadata,omitempty"`
	NotBefore int64      `json:"nbf"`
	Expires   int64      `json:"exp"`
	Video     videoGrant `json:"video"`
}

// ParticipantToken signs a LiveKit access token locally using the configured
// API key and secret. No separate focus/JWT service is required.
func ParticipantToken(apiKey, apiSecret, room, identity, name, metadata string, now time.Time) (string, error) {
	if apiKey == "" || apiSecret == "" || room == "" || identity == "" {
		return "", errors.New("LiveKit API key, secret, room, and identity are required")
	}
	header, err := encodeSegment(tokenHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := encodeSegment(tokenClaims{
		Issuer: apiKey, Subject: identity, Name: name, Metadata: metadata,
		NotBefore: now.Add(-10 * time.Second).Unix(), Expires: now.Add(2 * time.Hour).Unix(),
		Video: videoGrant{RoomJoin: true, Room: room, CanPublish: true, CanSubscribe: true},
	})
	if err != nil {
		return "", err
	}
	unsigned := header + "." + claims
	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func encodeSegment(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode JWT segment: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
