package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

const fingerprintPrefix = "sha256:"

var (
	fingerprintPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	checkedTaskPattern = regexp.MustCompile(`(?m)^(\s*)- \[[xX]\]`)
)

// Fingerprint computes the canonical content fingerprint of a document body.
// Normalization is deliberately minimal and versioned by design: CRLF and CR
// become LF, checked task markers (`- [x]` / `- [X]` as a line's first token)
// are fingerprinted as unchecked — checkbox state is execution progress owned
// by `task complete`, not approved content — then leading and trailing
// whitespace is trimmed. Changing this normalization re-fingerprints
// identical content and is a breaking change.
func Fingerprint(body string) string {
	sum := sha256.Sum256([]byte(normalizeBody(body)))
	return fingerprintPrefix + hex.EncodeToString(sum[:])
}

// ValidFingerprint reports whether value is a well-formed fingerprint.
func ValidFingerprint(value string) bool {
	return fingerprintPattern.MatchString(value)
}

// BodyMatchesFingerprint reports whether the body's fingerprint equals the
// recorded value. Malformed recorded values never match (fail-closed).
func BodyMatchesFingerprint(body, fingerprint string) bool {
	if !ValidFingerprint(fingerprint) {
		return false
	}
	return Fingerprint(body) == fingerprint
}

func normalizeBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = checkedTaskPattern.ReplaceAllString(normalized, "${1}- [ ]")
	return strings.TrimSpace(normalized)
}
