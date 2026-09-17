package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
	"github.com/andrearaponi/walden/internal/workflow"
)

func integrityRefresh(t *testing.T, root string) {
	t.Helper()
	result, err := workflow.Verify(context.Background(), root, "gate-demo", true, false, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
	if err != nil || len(result.Failed) != 0 {
		t.Fatalf("fixture refresh: %+v %v", result, err)
	}
}

func integrityStrict(t *testing.T, root, name string, opts Options) ReleaseReport {
	t.Helper()
	opts.Strict = true
	report, err := ReleaseCheck(context.Background(), root, name, opts)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

type integrityGitRunner func(context.Context, string, ...string) (shell.Response, error)

func (f integrityGitRunner) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

func TestEvidenceIntegrityCommittedReleaseInputs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required by this acceptance group")
	}
	t.Run("ignored inputs and committed reproducibility", func(t *testing.T) {
		root := gateRepo(t)
		gitIn(t, root, "rm", "-r", "--cached", ".walden")
		write(t, root, ".gitignore", ".walden/\nscratch/\n")
		gitIn(t, root, "add", ".gitignore")
		gitIn(t, root, "commit", "-qm", "local metadata")
		integrityRefresh(t, root)
		plain, err := ReleaseCheck(context.Background(), root, "gate-demo", Options{})
		if err != nil || !plain.Releasable() {
			t.Fatalf("non-strict fixture not green: %+v %v", plain, err)
		}
		blocked := integrityStrict(t, root, "gate-demo", Options{})
		if blocked.Releasable() || !strings.Contains(strings.Join(blocked.WorktreeBlockers, " "), ".walden/specs/gate-demo/requirements.md") {
			t.Fatalf("ignored uncommitted certification inputs were accepted: %+v", blocked)
		}
		gitIn(t, root, "add", "-f", ".walden")
		gitIn(t, root, "commit", "-qm", "same committed inputs")
		passed := integrityStrict(t, root, "", Options{})
		if !passed.Releasable() {
			t.Fatalf("committed equivalent failed: %+v", passed)
		}
		write(t, root, "scratch/cache", "irrelevant")
		if again := integrityStrict(t, root, "", Options{}); !again.Releasable() {
			t.Fatalf("ignored scratch changed input binding: %+v", again)
		}
		clone := filepath.Join(t.TempDir(), "checkout")
		gitIn(t, root, "clone", "-q", "--no-hardlinks", root, clone)
		gitIn(t, clone, "checkout", "-q", "--detach", passed.CertifiedCommit)
		if replay := integrityStrict(t, clone, "", Options{}); !replay.Releasable() || replay.CertifiedCommit != passed.CertifiedCommit {
			t.Fatalf("reported commit does not reproduce judgment: %+v", replay)
		}
	})
	t.Run("clean-status metadata mismatch", func(t *testing.T) {
		root := gateRepo(t)
		path := ".walden/specs/gate-demo/requirements.md"
		gitIn(t, root, "update-index", "--assume-unchanged", path)
		data, _ := os.ReadFile(filepath.Join(root, path))
		write(t, root, path, strings.Replace(string(data), "\n---\n", "\nx-local: changed\n---\n", 1))
		report := integrityStrict(t, root, "gate-demo", Options{})
		if report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), path) {
			t.Fatalf("clean status concealed input mismatch: %+v", report)
		}
	})
	t.Run("unborn HEAD and pending ledger absence", func(t *testing.T) {
		root := t.TempDir()
		gitIn(t, root, "init", "-q", "-b", "main")
		gitIn(t, root, "config", "user.email", "fixture@walden.test")
		gitIn(t, root, "config", "user.name", "Fixture")
		addFeature(t, root, "gate-demo", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
		write(t, root, ".git/info/exclude", ".walden/\n")
		waived := Options{AllowPending: true, WaiverReason: "fixture deferred work"}
		if report := integrityStrict(t, root, "gate-demo", waived); report.Releasable() {
			t.Fatal("unborn HEAD certified with a waiver")
		}
		gitIn(t, root, "add", "-f", ".walden")
		gitIn(t, root, "commit", "-qm", "pending plan, no ledger")
		if report := integrityStrict(t, root, "gate-demo", waived); !report.Releasable() {
			t.Fatalf("legitimate equal ledger absence blocked: %+v", report)
		}
	})
	t.Run("completed work requires its ledger", func(t *testing.T) {
		root := gateRepo(t)
		gitIn(t, root, "rm", ".walden/evidence/gate-demo.json")
		gitIn(t, root, "commit", "-qm", "missing required ledger")
		report := integrityStrict(t, root, "gate-demo", Options{AllowPending: true, WaiverReason: "cannot waive evidence"})
		if report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), ".walden/evidence/gate-demo.json") {
			t.Fatalf("required absence escaped input binding: %+v", report)
		}
	})
	t.Run("symlinked input is not a committed blob", func(t *testing.T) {
		root := gateRepo(t)
		path := ".walden/specs/gate-demo/requirements.md"
		data, _ := os.ReadFile(filepath.Join(root, path))
		target := filepath.Join(t.TempDir(), "external.md")
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, root, "update-index", "--assume-unchanged", path)
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
		if report := integrityStrict(t, root, "gate-demo", Options{}); report.Releasable() {
			t.Fatal("strict gate followed an external symlink")
		}
	})
	t.Run("committed portfolio cannot lose a hidden feature", func(t *testing.T) {
		root := gateRepo(t)
		addFeature(t, root, "hidden", certifiableRequirements(), certifiableDesign(""), certifiableTasks())
		gitIn(t, root, "add", ".walden")
		gitIn(t, root, "commit", "-qm", "second feature")
		for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
			gitIn(t, root, "update-index", "--skip-worktree", ".walden/specs/hidden/"+name)
		}
		if err := os.RemoveAll(filepath.Join(root, ".walden/specs/hidden")); err != nil {
			t.Fatal(err)
		}
		if report := integrityStrict(t, root, "", Options{}); report.Releasable() {
			t.Fatal("full portfolio silently omitted a committed feature")
		}
		if report := integrityStrict(t, root, "gate-demo", Options{}); !report.Releasable() {
			t.Fatalf("explicit feature selection was expanded: %+v", report)
		}
	})
	t.Run("unreadable input fails closed", func(t *testing.T) {
		root := gateRepo(t)
		previous := readInputFile
		t.Cleanup(func() { readInputFile = previous })
		readInputFile = func(path string) ([]byte, error) {
			if strings.HasSuffix(path, "requirements.md") {
				return nil, os.ErrPermission
			}
			return os.ReadFile(path)
		}
		if report := integrityStrict(t, root, "gate-demo", Options{}); report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), "requirements.md") {
			t.Fatalf("unreadable input accepted: %+v", report)
		}
	})
	t.Run("unreadable committed blob", func(t *testing.T) {
		root := gateRepo(t)
		previous := gitRunner
		t.Cleanup(func() { gitRunner = previous })
		gitRunner = integrityGitRunner(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			for _, arg := range args {
				if arg == "cat-file" {
					return shell.Response{ExitCode: 128, Stderr: "injected unreadable commit blob"}, nil
				}
			}
			return shell.NewExecRunner().Run(ctx, name, args...)
		})
		report := integrityStrict(t, root, "gate-demo", Options{})
		if report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), "committed blob unavailable") {
			t.Fatalf("unreadable committed bytes accepted: %+v", report)
		}
	})
	t.Run("symlinked input ancestor", func(t *testing.T) {
		root := gateRepo(t)
		path := filepath.Join(root, ".walden", "specs", "gate-demo")
		target := filepath.Join(t.TempDir(), "outside-feature")
		for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
			gitIn(t, root, "update-index", "--assume-unchanged", ".walden/specs/gate-demo/"+name)
		}
		if err := os.Rename(path, target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		report := integrityStrict(t, root, "gate-demo", Options{})
		if report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), "symbolic link") {
			t.Fatalf("external ancestor was followed: %+v", report)
		}
	})
	t.Run("judgment reads each captured input once", func(t *testing.T) {
		root := gateRepo(t)
		previous := readInputFile
		t.Cleanup(func() { readInputFile = previous })
		calls := map[string]int{}
		readInputFile = func(path string) ([]byte, error) {
			calls[path]++
			data, err := os.ReadFile(path)
			if strings.HasSuffix(path, "gate-demo.json") {
				write(t, root, ".walden/specs/gate-demo/requirements.md", "a later, different document")
			}
			return data, err
		}
		report := integrityStrict(t, root, "gate-demo", Options{})
		if len(calls) != 4 {
			t.Fatalf("expected exactly four captured files, got %v", calls)
		}
		for path, count := range calls {
			if count != 1 {
				t.Fatalf("%s read %d times", path, count)
			}
		}
		if !criterion(t, report.Features[0], "chain").Passed || !criterion(t, report.Features[0], "validation").Passed || !criterion(t, report.Features[0], "evidence").Passed {
			t.Fatalf("judgment consumed a later disk version: %+v", report.Features[0])
		}
	})
	t.Run("observed HEAD movement does not relabel the verdict", func(t *testing.T) {
		root := gateRepo(t)
		previous := gitRunner
		t.Cleanup(func() { gitRunner = previous })
		statuses := 0
		gitRunner = integrityGitRunner(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
			for _, arg := range args {
				if arg == "status" {
					statuses++
					if statuses == 2 {
						gitIn(t, root, "commit", "--allow-empty", "-qm", "concurrent head movement")
					}
				}
			}
			return shell.NewExecRunner().Run(ctx, name, args...)
		})
		report := integrityStrict(t, root, "gate-demo", Options{})
		if report.Releasable() || !strings.Contains(strings.Join(report.WorktreeBlockers, " "), "HEAD changed") {
			t.Fatalf("HEAD movement accepted: %+v", report)
		}
	})
}
