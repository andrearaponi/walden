package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/templates"
)

type bootstrapFile struct {
	Source string
	Target string
	// RefreshPin marks the generated workflow, the one bootstrap file whose
	// existing content repo init edits: the version of its install targets.
	RefreshPin bool
}

// InitReport describes the file-level outcome of repository bootstrap.
type InitReport struct {
	CreatedFiles          []string
	UpdatedFiles          []string
	SkippedFiles          []string
	GitAlreadyInitialized bool
	GitInitialized        bool
	// Warnings are non-blocking notes about files the run left as they were.
	Warnings []string
}

var repoBootstrapFiles = []bootstrapFile{
	{Source: "walden/constitution.md", Target: ".walden/constitution.md"},
	{Source: "walden/gitignore", Target: ".walden/.gitignore"},
	{Source: "walden/lessons.md", Target: ".walden/lessons.md"},
	{Source: "github/pull_request_template.md", Target: ".github/pull_request_template.md"},
	{Source: "github/workflows/validate-walden.yml", Target: ".github/workflows/validate-walden.yml", RefreshPin: true},
}

var gitInitRunner = runGitInit

// versionToken marks where bootstrap templates receive the generating
// binary's version, so generated CI installs the exact CLI that wrote it.
const versionToken = "{{WALDEN_VERSION}}"

// noInstallTargetWarning tells the user why an existing workflow kept its pin
// and how to get the current template back.
const noInstallTargetWarning = "%s names no walden install target, so its pin was not refreshed: delete the file and run walden repo init again to regenerate it from the current template"

// Init bootstraps a repository with the baseline Walden file layout. It
// creates the bootstrap files that are missing and never overwrites one that
// exists. The one edit it makes to an existing file is the version of the
// workflow's install targets, set to the pin of this binary: a release
// version is an exact pin, a source build falls back to "latest".
func Init(root string, version string) (InitReport, error) {
	report := InitReport{}
	pin := installPin(version)

	// Read what the run may edit before it writes anything: a failure here
	// leaves the repository as it was, git and directories included.
	existing, err := readRefreshable(root)
	if err != nil {
		return InitReport{}, err
	}

	gitState, err := ensureGitRepository(root)
	if err != nil {
		return InitReport{}, err
	}
	report.GitAlreadyInitialized = gitState.AlreadyInitialized
	report.GitInitialized = gitState.Initialized

	if err := os.MkdirAll(filepath.Join(root, ".walden", "specs"), 0o755); err != nil {
		return InitReport{}, fmt.Errorf("create .walden/specs: %w", err)
	}

	repoFS := templates.RepoFS()

	for _, file := range repoBootstrapFiles {
		content, err := fs.ReadFile(repoFS, file.Source)
		if err != nil {
			return InitReport{}, fmt.Errorf("read bootstrap template %q: %w", file.Source, err)
		}
		content = []byte(strings.ReplaceAll(string(content), versionToken, pin))

		created, err := createFile(root, file.Target, content)
		if err != nil {
			return InitReport{}, err
		}
		if created {
			report.CreatedFiles = append(report.CreatedFiles, file.Target)
			continue
		}

		// The file exists. It is kept as it is unless its content was read
		// for a pin refresh; one that appeared after that read waits for the
		// next run.
		current, read := existing[file.Target]
		if !read {
			report.SkippedFiles = append(report.SkippedFiles, file.Target)
			continue
		}

		refreshed, found := refreshPin(current, pin)
		switch {
		case !found:
			report.SkippedFiles = append(report.SkippedFiles, file.Target)
			report.Warnings = append(report.Warnings, fmt.Sprintf(noInstallTargetWarning, file.Target))
		case bytes.Equal(refreshed, current):
			report.SkippedFiles = append(report.SkippedFiles, file.Target)
		default:
			if err := replaceFile(root, file.Target, refreshed); err != nil {
				return InitReport{}, err
			}
			report.UpdatedFiles = append(report.UpdatedFiles, file.Target)
		}
	}

	return report, nil
}

// installPin maps the generating binary's version to the go install target.
// Only an exact vMAJOR.MINOR.PATCH release tag is pinnable: dev builds and
// git-describe suffixed versions (v0.7.1-2-gabc1234, what setup.sh stamps on
// clone builds) are not resolvable by `go install` and fall back to latest.
func installPin(version string) string {
	if isReleaseTag(version) {
		return version
	}
	return "latest"
}

// installTarget matches the module path of the walden command and the
// version after it. A version starts and ends with a letter or a digit and
// holds letters, digits and . _ + / - in between, so neither the shell
// syntax that can follow it (a space, a quote, ; & | or a closing
// parenthesis) nor the full stop of a sentence is taken for part of it.
var installTarget = regexp.MustCompile(`(github\.com/andrearaponi/walden/cmd/walden@)[0-9A-Za-z](?:[0-9A-Za-z._+/-]*[0-9A-Za-z])?`)

// refreshPin returns the workflow with the version of every install target
// set to pin, and whether the workflow names an install target at all. Only
// the versions change: every other byte of the workflow is returned as it is.
func refreshPin(workflow []byte, pin string) (refreshed []byte, found bool) {
	if !installTarget.Match(workflow) {
		return workflow, false
	}

	// A dollar sign in the replacement would be read as a group reference.
	literalPin := strings.ReplaceAll(pin, "$", "$$")
	return installTarget.ReplaceAll(workflow, []byte("${1}"+literalPin)), true
}

func isReleaseTag(version string) bool {
	if !strings.HasPrefix(version, "v") {
		return false
	}
	parts := strings.Split(version[1:], ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if value, err := strconv.Atoi(part); err != nil || value < 0 {
			return false
		}
	}
	return true
}

type gitRepositoryState struct {
	AlreadyInitialized bool
	Initialized        bool
}

func ensureGitRepository(root string) (gitRepositoryState, error) {
	gitPath := filepath.Join(root, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := gitInitRunner(root); err != nil {
				return gitRepositoryState{}, fmt.Errorf("initialize git repository: %w", err)
			}
			return gitRepositoryState{Initialized: true}, nil
		}

		return gitRepositoryState{}, fmt.Errorf("inspect git metadata: %w", err)
	}

	return gitRepositoryState{AlreadyInitialized: true}, nil
}

func runGitInit(root string) error {
	candidates := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "init"},
	}

	var failures []string
	for _, candidate := range candidates {
		cmd := exec.Command(candidate[0], candidate[1:]...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		failures = append(failures, fmt.Sprintf("%s: %s", strings.Join(candidate, " "), message))
	}

	return errors.New(strings.Join(failures, "; "))
}

// readRefreshable returns the current content of every existing bootstrap
// file whose pin a run refreshes. It is the only read of an existing
// bootstrap file, and it happens before any write.
func readRefreshable(root string) (map[string][]byte, error) {
	existing := map[string][]byte{}
	for _, file := range repoBootstrapFiles {
		if !file.RefreshPin {
			continue
		}

		content, err := os.ReadFile(filepath.Join(root, file.Target))
		switch {
		case err == nil:
			existing[file.Target] = content
		case errors.Is(err, os.ErrNotExist):
			// Missing: the run creates it from the template.
		default:
			return nil, fmt.Errorf("read %q: %w", file.Target, err)
		}
	}

	return existing, nil
}

// createFile writes content to a target where nothing exists yet. It reports
// false, writing nothing, when the file is already there. Creation is
// exclusive, so no bootstrap file is opened for writing once it exists and a
// file that appears during the run cannot be truncated.
func createFile(root, target string, content []byte) (bool, error) {
	path := filepath.Join(root, target)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create parent directory for %q: %w", target, err)
	}

	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create %q: %w", target, err)
	}

	_, writeErr := handle.Write(content)
	if closeErr := handle.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		// The file is this run's own: a partial template must not stay to
		// be taken, by the next run, for a file the user wrote.
		_ = os.Remove(path)
		return false, fmt.Errorf("create %q: %w", target, writeErr)
	}

	return true, nil
}

// replaceFile swaps the content of an existing target atomically, so a write
// that fails leaves the previous bytes, and gives the new file the
// permission bits the previous one had.
func replaceFile(root, target string, content []byte) error {
	path := filepath.Join(root, target)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("update %q: %w", target, err)
	}
	if err := spec.WriteFileAtomic(path, content); err != nil {
		return fmt.Errorf("update %q: %w", target, err)
	}
	if err := os.Chmod(path, info.Mode().Perm()); err != nil {
		return fmt.Errorf("update %q: %w", target, err)
	}

	return nil
}
