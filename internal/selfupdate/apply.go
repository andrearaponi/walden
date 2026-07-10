package selfupdate

import (
	"context"
	"fmt"
	"os"

	"github.com/andrearaponi/walden/internal/skilldist"
)

// SkillSync is one skill installation re-synced by an update.
type SkillSync struct {
	Agent string
	Scope string
	Path  string
}

// Report describes a completed or short-circuited update run.
type Report struct {
	PreviousVersion  string
	InstalledVersion string
	ExecutablePath   string
	ReleaseNotesURL  string
	SyncedSkills     []SkillSync
	Warnings         []string
	AlreadyUpToDate  bool
}

// Apply runs the full update flow: resolve, compare, snapshot skills, probe,
// download, verify, swap, smoke-test (with rollback), re-sync skills, report.
// Every abort path cleans its staging file; after the swap the previous
// binary survives as a backup until the smoke test passes.
func Apply(ctx context.Context, opts Options) (Report, error) {
	status, err := Check(opts)
	if err != nil {
		return Report{}, err
	}
	if !status.UpdateAvailable {
		return Report{
			PreviousVersion:  status.CurrentVersion,
			InstalledVersion: status.CurrentVersion,
			AlreadyUpToDate:  true,
		}, nil
	}

	executable := opts.ExecutablePath
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			return Report{}, fmt.Errorf("locate running binary: %w", err)
		}
	}
	executable, err = resolveExecutable(executable)
	if err != nil {
		return Report{}, err
	}

	// Snapshot before any mutation: the new binary reinstalls these slots.
	slots := snapshotSkillSlots(skilldist.Options{
		Version: opts.CurrentVersion,
		WorkDir: opts.WorkDir,
		Env:     opts.Env,
	})

	staged, err := probeStaging(executable)
	if err != nil {
		return Report{}, err
	}

	asset := assetName(status.TargetVersion, opts.OS, opts.Arch)
	digest, err := downloadAsset(opts.HTTPClient, opts.BaseURL, status.TargetVersion, asset, staged)
	if err != nil {
		_ = os.Remove(staged)
		return Report{}, err
	}

	if err := verifyChecksum(opts.HTTPClient, opts.BaseURL, status.TargetVersion, asset, staged, digest); err != nil {
		return Report{}, err
	}

	backup, err := swapExecutable(staged, executable)
	if err != nil {
		_ = os.Remove(staged)
		return Report{}, err
	}

	if err := smokeTestAndFinalize(ctx, opts.Runner, executable, backup, status.TargetVersion); err != nil {
		return Report{}, err
	}

	synced, warnings := resyncSkills(ctx, opts.Runner, executable, status.TargetVersion, slots)

	report := Report{
		PreviousVersion:  status.CurrentVersion,
		InstalledVersion: status.TargetVersion,
		ExecutablePath:   executable,
		ReleaseNotesURL:  fmt.Sprintf("%s/releases/tag/%s", opts.BaseURL, status.TargetVersion),
		SyncedSkills:     make([]SkillSync, 0, len(synced)),
		Warnings:         warnings,
	}
	for _, slot := range synced {
		report.SyncedSkills = append(report.SyncedSkills, SkillSync{
			Agent: slot.Agent,
			Scope: string(slot.Scope),
			Path:  slot.Path,
		})
	}
	return report, nil
}
