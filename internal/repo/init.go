package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andrearaponi/walden/templates"
)

type bootstrapFile struct {
	Source     string
	Target     string
	Executable bool
}

// InitReport describes the file-level outcome of repository bootstrap.
type InitReport struct {
	CreatedFiles          []string
	UpdatedFiles          []string
	SkippedFiles          []string
	GitAlreadyInitialized bool
	GitInitialized        bool
}

var repoBootstrapFiles = []bootstrapFile{
	{Source: "walden/constitution.md", Target: ".walden/constitution.md"},
	{Source: "walden/lessons.md", Target: ".walden/lessons.md"},
	{Source: "github/pull_request_template.md", Target: ".github/pull_request_template.md"},
	{Source: "github/workflows/validate-walden.yml", Target: ".github/workflows/validate-walden.yml"},
}

var gitInitRunner = runGitInit

// Init bootstraps a repository with the baseline Walden file layout.
func Init(root string) (InitReport, error) {
	report := InitReport{}

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

		state, err := writeManagedFile(root, file, content)
		if err != nil {
			return InitReport{}, err
		}

		switch state {
		case "created":
			report.CreatedFiles = append(report.CreatedFiles, file.Target)
		case "updated":
			report.UpdatedFiles = append(report.UpdatedFiles, file.Target)
		default:
			report.SkippedFiles = append(report.SkippedFiles, file.Target)
		}
	}

	return report, nil
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

func writeManagedFile(root string, file bootstrapFile, content []byte) (string, error) {
	targetPath := filepath.Join(root, file.Target)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create parent directory for %q: %w", file.Target, err)
	}

	mode := fs.FileMode(0o644)
	if file.Executable {
		mode = 0o755
	}

	existing, err := os.ReadFile(targetPath)
	switch {
	case err == nil:
		if string(existing) == string(content) {
			if chmodErr := os.Chmod(targetPath, mode); chmodErr != nil {
				return "", fmt.Errorf("set mode for %q: %w", file.Target, chmodErr)
			}

			return "skipped", nil
		}

		if writeErr := os.WriteFile(targetPath, content, mode); writeErr != nil {
			return "", fmt.Errorf("update %q: %w", file.Target, writeErr)
		}
		return "updated", nil
	case errors.Is(err, os.ErrNotExist):
		if writeErr := os.WriteFile(targetPath, content, mode); writeErr != nil {
			return "", fmt.Errorf("create %q: %w", file.Target, writeErr)
		}
		return "created", nil
	default:
		return "", fmt.Errorf("read %q: %w", file.Target, err)
	}
}
