package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Probe is one declared environment probe: a named command whose trimmed
// output joins the execution profile of every evidence record produced by a
// run. Names are lowercase kebab; `platform` and `walden` are reserved for
// the kernel's own profile entries.
type Probe struct {
	Name string
	Argv []string
}

// probeLinePattern matches a declaration list item. In environment.md, list
// items ARE probes by contract; headings and prose are documentation.
var probeLinePattern = regexp.MustCompile(`^- ([a-z0-9-]+): (\[.+\])\s*$`)

var reservedProbeNames = map[string]bool{
	"platform": true,
	"walden":   true,
}

// EnvironmentProbesPath returns the declaration file location for a root.
func EnvironmentProbesPath(root string) string {
	return filepath.Join(root, ".walden", "environment.md")
}

// LoadEnvironmentProbes parses `.walden/environment.md` into the declared
// probe list. An absent file means no probes — the platform-only profile.
// List items must parse as `- <name>: ["cmd", "arg", …]` (the proof-step
// argv format); anything that is not a list item is ignored, so the file
// self-documents. Malformed items, reserved names, and duplicates error
// naming the file and line.
func LoadEnvironmentProbes(root string) ([]Probe, error) {
	path := EnvironmentProbesPath(root)
	text, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	probes := []Probe{}
	seen := map[string]bool{}
	for index, line := range strings.Split(string(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		match := probeLinePattern.FindStringSubmatch(trimmed)
		if match == nil {
			return nil, fmt.Errorf("%s:%d: malformed probe line: expected `- <name>: [\"cmd\", …]` with a lowercase kebab name", path, index+1)
		}
		name := match[1]
		if reservedProbeNames[name] {
			return nil, fmt.Errorf("%s:%d: probe name %q is reserved for the kernel profile", path, index+1, name)
		}
		if seen[name] {
			return nil, fmt.Errorf("%s:%d: duplicate probe name %q", path, index+1, name)
		}
		argv, err := parseVerificationArgv(match[2], index)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
		seen[name] = true
		probes = append(probes, Probe{Name: name, Argv: argv})
	}
	return probes, nil
}
