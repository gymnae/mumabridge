package matrix

import (
	"strings"
	"testing"
)

func TestGhostLocalpartStableAndSafe(t *testing.T) {
	first := GhostLocalpart("bridge_", "mumble-session-42", "Arnold / Admin")
	second := GhostLocalpart("bridge_", "mumble-session-42", "Arnold / Admin")
	if first != second {
		t.Fatalf("identity is not stable: %q != %q", first, second)
	}
	if first != "bridge_arnold___admin_eb2c854a345c" {
		t.Fatalf("unexpected localpart %q", first)
	}
	if strings.ContainsAny(first, " /:@") {
		t.Fatalf("unsafe localpart %q", first)
	}
}

func TestGhostLocalpartAvoidsNormalizedNameCollision(t *testing.T) {
	a := GhostLocalpart("bridge_", "session-a", "same")
	b := GhostLocalpart("bridge_", "session-b", "same")
	if a == b {
		t.Fatal("different stable IDs collided")
	}
}

func TestIsGhostUser(t *testing.T) {
	if !IsGhostUser("@bridge_arnold:example.org", "bridge_") {
		t.Fatal("expected bridge user to be detected")
	}
	if IsGhostUser("@arnold:example.org", "bridge_") {
		t.Fatal("ordinary user detected as ghost")
	}
}
