package skilldist

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Uninstall removes the skill for agent at scope. A missing installation is
// a no-op reported via Report.NotInstalled, never an error.
func Uninstall(agent Agent, scope Scope, opts Options) (Report, error) {
	target, err := resolveTarget(agent, scope, opts)
	if err != nil {
		return Report{}, err
	}

	report := Report{Agent: agent.Name, Scope: scope, Path: target}

	switch agent.Kind {
	case KindFile:
		removed, err := removeSkillFile(target)
		if err != nil {
			return Report{}, err
		}
		report.NotInstalled = !removed
	case KindBlock:
		removed, err := removeBlock(target)
		if err != nil {
			return Report{}, err
		}
		report.NotInstalled = !removed
	default:
		return Report{}, fmt.Errorf("agent %s: unsupported write kind %q", agent.Name, agent.Kind)
	}

	report.LegacyRemoved = cleanupLegacy(agent, scope, opts)
	return report, nil
}

// UninstallAll removes the user-scope skill for every supported agent in
// registry order, stopping on the first failure.
func UninstallAll(opts Options) ([]Report, error) {
	reports := make([]Report, 0, len(agents))
	for _, agent := range agents {
		report, err := Uninstall(agent, ScopeUser, opts)
		if err != nil {
			return reports, fmt.Errorf("uninstall %s: %w", agent.Name, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// removeSkillFile removes the dedicated skill directory (skills/walden/)
// that holds SKILL.md.
func removeSkillFile(target string) (removed bool, err error) {
	dir := filepath.Dir(target)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect %s: %w", dir, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, fmt.Errorf("remove %s: %w", dir, err)
	}
	return true, nil
}

// removeBlock extracts the Walden marker block from target, preserving all
// unrelated content. A file left empty by the extraction is deleted.
func removeBlock(target string) (removed bool, err error) {
	existing, err := os.ReadFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", target, err)
	}

	begin, end, err := blockBounds(existing, target)
	if err != nil {
		return false, err
	}
	if begin == -1 {
		return false, nil
	}

	// Also drop the blank separator line inserted ahead of the block.
	start := begin
	if start >= 2 && existing[start-1] == '\n' && existing[start-2] == '\n' {
		start--
	}

	out := append([]byte(nil), existing[:start]...)
	out = append(out, existing[end:]...)

	if len(bytes.TrimSpace(out)) == 0 {
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("remove %s: %w", target, err)
		}
		return true, nil
	}
	if _, err := writeFileAtomic(target, out); err != nil {
		return false, err
	}
	return true, nil
}
