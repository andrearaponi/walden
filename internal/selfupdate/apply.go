package selfupdate

import (
	"context"
	"fmt"
	"os"
)

// Report describes a completed or short-circuited update run.
type Report struct {
	PreviousVersion  string
	InstalledVersion string
	ExecutablePath   string
	ReleaseNotesURL  string
	Warnings         []string
	AlreadyUpToDate  bool
}

// Apply runs the full update flow: resolve, compare, probe, download, verify,
// swap, smoke-test (with rollback), report. It replaces one file: the binary.
// Every abort path cleans its staging file; after the swap the previous
// binary survives as a backup until the smoke test passes.
func Apply(ctx context.Context, opts Options) (Report, error) {
	if err := guardOS(opts); err != nil {
		return Report{}, err
	}
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

	return Report{
		PreviousVersion:  status.CurrentVersion,
		InstalledVersion: status.TargetVersion,
		ExecutablePath:   executable,
		ReleaseNotesURL:  fmt.Sprintf("%s/releases/tag/%s", opts.BaseURL, status.TargetVersion),
		Warnings:         []string{},
	}, nil
}
