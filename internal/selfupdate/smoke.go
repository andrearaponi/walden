package selfupdate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
)

// smokeTestAndFinalize proves the freshly installed binary works by running
// its version command: pass means exit 0 with the target tag in the output.
// A pass deletes the backup; any failure restores the previous binary over
// the executable path so the install never ends in a broken state.
func smokeTestAndFinalize(ctx context.Context, runner shell.Runner, executable, backup, targetTag string) error {
	resp, err := runner.Run(ctx, executable, "version")

	pass := err == nil && resp.ExitCode == 0 && strings.Contains(resp.Stdout, targetTag)
	if pass {
		if removeErr := os.Remove(backup); removeErr != nil {
			return fmt.Errorf("update installed, but removing the backup failed: %w (remove %s manually)", removeErr, backup)
		}
		return nil
	}

	reason := describeSmokeFailure(resp, err, targetTag)
	if restoreErr := os.Rename(backup, executable); restoreErr != nil {
		return fmt.Errorf("smoke test failed (%s) and restoring the previous binary failed: %v (backup kept at %s)", reason, restoreErr, backup)
	}
	return fmt.Errorf("smoke test failed for the new binary (%s); previous binary restored", reason)
}

func describeSmokeFailure(resp shell.Response, err error, targetTag string) string {
	switch {
	case err != nil:
		return fmt.Sprintf("could not execute it: %v", err)
	case resp.ExitCode != 0:
		return fmt.Sprintf("version exited %d: %s", resp.ExitCode, strings.TrimSpace(resp.Stderr))
	default:
		return fmt.Sprintf("version output %q does not report %s", strings.TrimSpace(resp.Stdout), targetTag)
	}
}
