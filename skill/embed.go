// Package skill exposes the canonical Walden AI skill bundled into the
// binary, so every distribution channel that delivers the CLI also delivers
// the skill at the matching version.
package skill

import (
	_ "embed"
)

//go:embed walden/SKILL.md
var content []byte

// Content returns the embedded canonical SKILL.md bytes.
func Content() []byte {
	return append([]byte(nil), content...)
}
