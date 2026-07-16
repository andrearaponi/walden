package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
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
// become LF; in tasks.md — and only there — checked task markers (`- [x]` /
// `- [X]` as a line's first token) are fingerprinted as unchecked, because
// checkbox state there is execution progress owned by `task complete`, not
// approved content; in every other document a checkbox is content. Then
// leading and trailing whitespace is trimmed. Changing this normalization
// re-fingerprints identical content and is a breaking change.
func Fingerprint(documentPath, body string) string {
	sum := sha256.Sum256([]byte(normalizeBody(documentPath, body)))
	return fingerprintPrefix + hex.EncodeToString(sum[:])
}

// ValidFingerprint reports whether value is a well-formed fingerprint.
func ValidFingerprint(value string) bool {
	return fingerprintPattern.MatchString(value)
}

// BodyMatchesFingerprint reports whether the body's fingerprint equals the
// recorded value. Malformed recorded values never match (fail-closed).
func BodyMatchesFingerprint(documentPath, body, fingerprint string) bool {
	if !ValidFingerprint(fingerprint) {
		return false
	}
	return Fingerprint(documentPath, body) == fingerprint
}

func normalizeBody(documentPath, body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if filepath.Base(documentPath) == "tasks.md" {
		normalized = checkedTaskPattern.ReplaceAllString(normalized, "${1}- [ ]")
	}
	return strings.TrimSpace(normalized)
}
