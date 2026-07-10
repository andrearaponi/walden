package selfupdate

import (
	"context"
	"fmt"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/skilldist"
)

// skillInstallGate is the first release whose binary ships `skill install`.
// Targets below it cannot re-sync skills and degrade to a warning.
var skillInstallGate = releaseTag{0, 5, 0}

// skillSlot identifies one installed skill installation to re-sync.
type skillSlot struct {
	Agent string
	Scope skilldist.Scope
	Path  string
}

// snapshotSkillSlots records every skill installation detectable from the
// current environment. It runs before the swap so the update knows what to
// re-sync even though installation itself is delegated to the new binary.
func snapshotSkillSlots(opts skilldist.Options) []skillSlot {
	statuses, _ := skilldist.Status(opts)

	slots := make([]skillSlot, 0, len(statuses))
	for _, status := range statuses {
		if !status.Installed {
			continue
		}
		slots = append(slots, skillSlot{Agent: status.Agent, Scope: status.Scope, Path: status.Path})
	}
	return slots
}

// resyncSkills reinstalls each recorded slot by invoking the new binary's
// `skill install` command, so every installation carries the embedded skill
// matching the installed release. Failures never fail the update: the binary
// swap is the contract, skill drift stays repairable and is reported as
// warnings.
func resyncSkills(ctx context.Context, runner shell.Runner, executable, targetTag string, slots []skillSlot) ([]skillSlot, []string) {
	target, err := parseReleaseTag(targetTag)
	if err == nil && target.less(skillInstallGate) {
		return nil, []string{fmt.Sprintf("release %s predates `walden skill install`; skill re-sync skipped — run the installer to refresh skills for this release", targetTag)}
	}

	synced := make([]skillSlot, 0, len(slots))
	warnings := []string{}
	for _, slot := range slots {
		args := []string{"skill", "install", slot.Agent}
		if slot.Scope == skilldist.ScopeProject {
			args = append(args, "--project")
		}

		resp, runErr := runner.Run(ctx, executable, args...)
		if runErr != nil || resp.ExitCode != 0 {
			warnings = append(warnings, fmt.Sprintf("skill re-sync failed for %s (%s scope): %s — run `walden skill install %s` to repair", slot.Agent, slot.Scope, describeSyncFailure(resp, runErr), slot.Agent))
			continue
		}
		synced = append(synced, slot)
	}
	return synced, warnings
}

func describeSyncFailure(resp shell.Response, err error) string {
	if err != nil {
		return err.Error()
	}
	detail := strings.TrimSpace(resp.Stderr)
	if detail == "" {
		detail = fmt.Sprintf("exit %d", resp.ExitCode)
	}
	return detail
}
