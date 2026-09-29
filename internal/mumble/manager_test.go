package mumble

import (
	"strings"
	"testing"
)

func TestGhostNameStableAndMarked(t *testing.T) {
	name := GhostName("[mx] ", "Arnold", "@arnold:example.org")
	if !strings.HasPrefix(name, "[mx] Arnold~") {
		t.Fatalf("unexpected name %q", name)
	}
	if name != GhostName("[mx] ", "Arnold", "@arnold:example.org") {
		t.Fatal("ghost name is not stable")
	}
	if !IsBridgeOwned(name, "[mx] ") {
		t.Fatal("ghost marker was not detected")
	}
}

func TestGhostNameDisambiguatesUsers(t *testing.T) {
	if GhostName("[mx] ", "Arnold", "one") == GhostName("[mx] ", "Arnold", "two") {
		t.Fatal("stable IDs did not disambiguate names")
	}
}
