package matrix

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// GhostLocalpart returns a stable, Matrix-safe localpart for a Mumble user.
// The digest prevents collisions after normalization and avoids exposing a raw
// Mumble identifier as an account name.
func GhostLocalpart(prefix, stableID, displayName string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(displayName) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			normalized.WriteRune(r)
		} else {
			normalized.WriteByte('_')
		}
	}
	name := strings.Trim(normalized.String(), "_")
	if name == "" {
		name = "user"
	}
	if len(name) > 24 {
		name = name[:24]
	}
	digest := sha256.Sum256([]byte(stableID))
	return prefix + name + "_" + hex.EncodeToString(digest[:6])
}

func IsGhostUser(userID, prefix string) bool {
	if !strings.HasPrefix(userID, "@") {
		return false
	}
	localpart := strings.TrimPrefix(strings.SplitN(userID, ":", 2)[0], "@")
	return strings.HasPrefix(localpart, prefix)
}
