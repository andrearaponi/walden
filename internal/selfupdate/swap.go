package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveExecutable resolves the running binary's path through symbolic
// links, so the swap replaces the real target rather than the link.
func resolveExecutable(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve executable path %s: %w", path, err)
	}
	return resolved, nil
}

// probeStaging creates the PID-suffixed staging file next to the executable
// and returns its path. Staging in the same directory keeps the final rename
// atomic and doubles as the writability probe: it fails before any download
// when the install location cannot be written.
func probeStaging(executable string) (string, error) {
	dir := filepath.Dir(executable)
	staged := filepath.Join(dir, fmt.Sprintf(".walden-update-%d", os.Getpid()))

	file, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("cannot write to %s: %w (reinstall walden to a writable location, or rerun with the required permissions)", dir, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	return staged, nil
}

// swapExecutable installs the staged binary over the executable path with an
// atomic rename, keeping the previous binary as a PID-suffixed backup for
// rollback. The caller deletes the backup after a passing smoke test or
// restores it on failure.
func swapExecutable(staged, executable string) (string, error) {
	if err := os.Chmod(staged, 0o755); err != nil {
		return "", fmt.Errorf("mark staged binary executable: %w", err)
	}

	backup := filepath.Join(filepath.Dir(executable), fmt.Sprintf(".walden-backup-%d", os.Getpid()))
	if err := os.Rename(executable, backup); err != nil {
		return "", fmt.Errorf("back up current binary: %w", err)
	}

	if err := os.Rename(staged, executable); err != nil {
		// Put the previous binary back so a failed swap never leaves the
		// install without a working executable.
		_ = os.Rename(backup, executable)
		return "", fmt.Errorf("install new binary: %w", err)
	}
	return backup, nil
}
