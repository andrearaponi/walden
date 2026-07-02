package skilldist

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrearaponi/walden/skill"
)

// Options carries the context every skill operation needs. Injecting it
// keeps operations pure with respect to the process environment.
type Options struct {
	// Version is stamped into installed content (the binary's version).
	Version string
	// WorkDir anchors project-scope targets.
	WorkDir string
	// Env resolves user-scope targets.
	Env Env
}

// Report describes the outcome of one install or uninstall operation.
type Report struct {
	Agent         string
	Scope         Scope
	Path          string
	Replaced      bool
	NotInstalled  bool
	LegacyRemoved bool
}

// Install places the embedded skill for agent at scope, stamped with the
// binary version. Existing installations are replaced, never duplicated.
func Install(agent Agent, scope Scope, opts Options) (Report, error) {
	target, err := resolveTarget(agent, scope, opts)
	if err != nil {
		return Report{}, err
	}

	report := Report{Agent: agent.Name, Scope: scope, Path: target}
	content := Stamp(skill.Content(), opts.Version)

	switch agent.Kind {
	case KindFile:
		replaced, err := writeFileAtomic(target, content)
		if err != nil {
			return Report{}, err
		}
		report.Replaced = replaced
	case KindBlock:
		replaced, err := upsertBlock(target, content)
		if err != nil {
			return Report{}, err
		}
		report.Replaced = replaced
	default:
		return Report{}, fmt.Errorf("agent %s: unsupported write kind %q", agent.Name, agent.Kind)
	}

	report.LegacyRemoved = cleanupLegacy(agent, scope, opts)
	return report, nil
}

// InstallAll installs the skill at user scope for every supported agent in
// registry order, stopping on the first failure. The returned reports cover
// the agents that completed before the failure.
func InstallAll(opts Options) ([]Report, error) {
	reports := make([]Report, 0, len(agents))
	for _, agent := range agents {
		report, err := Install(agent, ScopeUser, opts)
		if err != nil {
			return reports, fmt.Errorf("install %s: %w", agent.Name, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func resolveTarget(agent Agent, scope Scope, opts Options) (string, error) {
	switch scope {
	case ScopeProject:
		target, ok := agent.ProjectTarget(opts.WorkDir)
		if !ok {
			return "", fmt.Errorf("agent %s supports only user scope", agent.Name)
		}
		return target, nil
	default:
		return agent.UserTarget(opts.Env)
	}
}

// writeFileAtomic writes content to target via a temp file and rename, so a
// failure never leaves a partially written target.
func writeFileAtomic(target string, content []byte) (replaced bool, err error) {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create target directory: %w", err)
	}

	_, statErr := os.Stat(target)
	replaced = statErr == nil

	tmp, err := os.CreateTemp(dir, ".walden-skill-*")
	if err != nil {
		return false, fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return false, fmt.Errorf("set permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return false, fmt.Errorf("replace target: %w", err)
	}
	return replaced, nil
}

// cleanupLegacy removes obsolete user-scope artifacts (e.g. the legacy
// /walden command file) and reports whether anything was removed.
func cleanupLegacy(agent Agent, scope Scope, opts Options) bool {
	if scope != ScopeUser || agent.LegacyUserFiles == nil {
		return false
	}
	removed := false
	for _, legacy := range agent.LegacyUserFiles(opts.Env) {
		if removeIfExists(legacy) {
			removed = true
		}
	}
	return removed
}

func removeIfExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return os.Remove(path) == nil
}
