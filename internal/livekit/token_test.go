package livekit

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParticipantTokenContainsRoomGrant(t *testing.T) {
	token, err := ParticipantToken("key", "secret", "!room:example", "ghost-1", "Arnold", `{"bridge":"mumabridge"}`, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid JWT shape: %q", token)
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims tokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Issuer != "key" || claims.Subject != "ghost-1" || claims.Video.Room != "!room:example" || !claims.Video.RoomJoin {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}
