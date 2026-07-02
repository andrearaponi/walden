package skilldist

import (
	"bytes"
	"fmt"
)

// The version marker records which binary version installed the skill. It is
// an HTML comment appended as the final line, so agents ignore it and the
// YAML frontmatter stays byte-identical to the embedded skill.
const (
	stampPrefix = "<!-- walden-skill-version: "
	stampSuffix = " -->"
)

// Stamp appends a version marker recording the installing binary's version.
// The original content is only ever extended, never modified.
func Stamp(content []byte, version string) []byte {
	stamped := append([]byte(nil), content...)
	if len(stamped) > 0 && !bytes.HasSuffix(stamped, []byte("\n")) {
		stamped = append(stamped, '\n')
	}
	return append(stamped, fmt.Sprintf("%s%s%s\n", stampPrefix, version, stampSuffix)...)
}

// Strip removes a trailing version marker, returning the body and the
// recorded version. Content without a marker (legacy installs) is returned
// unchanged with an empty version.
func Strip(content []byte) ([]byte, string) {
	trimmed := bytes.TrimSuffix(content, []byte("\n"))
	idx := bytes.LastIndexByte(trimmed, '\n')
	line := trimmed[idx+1:]
	if !bytes.HasPrefix(line, []byte(stampPrefix)) || !bytes.HasSuffix(line, []byte(stampSuffix)) {
		return content, ""
	}
	version := string(line[len(stampPrefix) : len(line)-len(stampSuffix)])
	return content[:idx+1], version
}
