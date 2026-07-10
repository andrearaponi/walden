package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// TaskDefinitionFingerprint hashes the canonical definition of one task: id,
// title, requirement references, design references, and verification text.
// Checkbox state and sibling tasks never enter it, so execution progress and
// unrelated plan edits leave the fingerprint unmoved while any change to
// this task's own contract moves it.
func TaskDefinitionFingerprint(task *Task) string {
	fields := []string{
		task.ID,
		task.Title,
		strings.Join(task.Requirements, ","),
		strings.Join(task.DesignRefs, ","),
		task.Verification,
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	return "sha256:" + hex.EncodeToString(sum[:])
}
