package app

import (
	"path"
	"strings"
	"testing"
)

// The release gate judges a root below the top level by the rules it applies
// at the top level. An uncommitted path under the root's .walden/ warns in
// default mode and blocks under --strict; any other uncommitted path blocks,
// under the root or in another directory of the repository; strict
// certification of the portfolio is bound to the features the commit holds
// under the root. The gate names every path as git does, from the top level.
func TestReleaseGateBelowTopLevel(t *testing.T) {
	for _, layout := range rootLayouts {
		t.Run(layout.name, func(t *testing.T) {
			top, root := layoutRepo(t, layout.prefix, subdirectoryTasks, passingFiles)
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.1")
			integrityMustCLI(t, "task", "complete", subdirectoryFeature, "1.2")
			commitEverything(t, top, "evidence")
			fromTop := func(underRoot string) string { return path.Join(layout.prefix, underRoot) }
			ledger := fromTop(".walden/evidence/" + subdirectoryFeature + ".json")

			// The ledger, rewritten by verify, is the only uncommitted path.
			integrityMustCLI(t, "verify", subdirectoryFeature, "--task", "1.2")
			if status := integrityGit(t, top, "status", "--porcelain"); status != "M "+ledger {
				t.Fatalf("the fixture has other uncommitted paths: %q", status)
			}
			result, code := integrityCLI(t, "release", "check", subdirectoryFeature)
			if code != 0 || result.Release == nil || !result.Release.Releasable {
				t.Errorf("default mode: an uncommitted ledger blocks: exit %d, %s, blockers %v", code, result.Summary, releaseBlockers(result.Release))
			}
			if result.Release != nil {
				if dirty := result.Release.Worktree.WaldenDirty; len(dirty) != 1 || dirty[0] != ledger {
					t.Errorf("default mode: paths under .walden/ = %v, want %s alone", dirty, ledger)
				}
				if blockers := result.Release.Worktree.Blockers; len(blockers) != 0 {
					t.Errorf("default mode: worktree blockers = %v, want none", blockers)
				}
			}
			if warnings := strings.Join(result.Warnings, "\n"); !strings.Contains(warnings, "uncommitted under .walden/ (not blocking): "+ledger) {
				t.Errorf("default mode: no warning names the ledger: %q", warnings)
			}
			strict, code := integrityCLI(t, "release", "check", subdirectoryFeature, "--strict")
			if code == 0 || strict.Release == nil || strict.Release.Releasable {
				t.Errorf("--strict: an uncommitted ledger does not block: exit %d, %s", code, strict.Summary)
			}
			if blockers := releaseBlockers(strict.Release); !strings.Contains(blockers, "uncommitted under .walden/: "+ledger+" — commit it before certifying (--strict)") {
				t.Errorf("--strict: no blocker names the ledger:\n%s", blockers)
			}
			commitEverything(t, top, "ledger")

			// A modified file blocks, under the root and elsewhere.
			diagnosticsFile(t, root, "status.txt", "green\nand more\n")
			if layout.prefix != "" {
				diagnosticsFile(t, top, "other/file.txt", "changed\n")
			}
			modified, code := integrityCLI(t, "release", "check", subdirectoryFeature)
			if code == 0 || modified.Release == nil || modified.Release.Releasable {
				t.Errorf("a modified file does not block: exit %d, %s", code, modified.Summary)
			}
			blockers := releaseBlockers(modified.Release)
			wantBlockers := []string{"uncommitted: " + fromTop("status.txt") + " — commit it before certifying"}
			if layout.prefix != "" {
				wantBlockers = append(wantBlockers, "uncommitted: other/file.txt — commit it before certifying")
			}
			for _, want := range wantBlockers {
				if !strings.Contains(blockers, want) {
					t.Errorf("the blockers lack %q:\n%s", want, blockers)
				}
			}
			if modified.Release != nil && len(modified.Release.Worktree.WaldenDirty) != 0 {
				t.Errorf("a path outside .walden/ is counted as Walden state: %v", modified.Release.Worktree.WaldenDirty)
			}
			integrityGit(t, top, "checkout", "--", ".")

			// Strict certification of the portfolio, everything committed.
			portfolio, code := integrityCLI(t, "release", "check", "--strict")
			if code != 0 || portfolio.Release == nil || !portfolio.Release.Releasable {
				t.Errorf("strict certification of the portfolio does not pass on a committed tree: exit %d, %s\n%s", code, portfolio.Summary, releaseBlockers(portfolio.Release))
			}
			if portfolio.Release != nil {
				if binding := portfolio.Release.Worktree.InputBinding; binding != "matched" {
					t.Errorf("committed input binding = %q, want matched", binding)
				}
				listed := false
				for _, input := range portfolio.Release.Worktree.Inputs {
					if input.Path == ".walden/specs" {
						listed = true
						if input.State != "matched" {
							t.Errorf("the portfolio input reads %s: %s", input.State, input.Detail)
						}
					}
				}
				if !listed {
					t.Errorf("the verdict lists no portfolio input: %+v", portfolio.Release.Worktree.Inputs)
				}
			}

			// A feature directory the working tree holds and the commit
			// does not: the portfolio no longer matches the commit.
			diagnosticsFile(t, root, ".walden/specs/extra/requirements.md", releasableRequirements)
			diagnosticsFile(t, root, ".walden/specs/extra/design.md", releasableDesign)
			diagnosticsFile(t, root, ".walden/specs/extra/tasks.md", releasableTasks)
			grown, code := integrityCLI(t, "release", "check", "--strict")
			if code == 0 || grown.Release == nil || grown.Release.Releasable {
				t.Errorf("strict certification passed with a feature the commit does not hold: exit %d, %s", code, grown.Summary)
			}
			want := "certification input .walden/specs: different — judged portfolio [extra " + subdirectoryFeature + "] differs from committed feature inventory [" + subdirectoryFeature + "]"
			if blockers := releaseBlockers(grown.Release); !strings.Contains(blockers, want) {
				t.Errorf("the blockers lack %q:\n%s", want, blockers)
			}
		})
	}
}
