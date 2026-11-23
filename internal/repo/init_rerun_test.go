package repo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/templates"
)

// keptTargets are the bootstrap files a run never writes once they exist.
var keptTargets = []string{
	".walden/constitution.md",
	".walden/lessons.md",
	".walden/.gitignore",
	".github/pull_request_template.md",
}

// bootstrapSources maps every bootstrap file to its embedded template. The
// table is written out here so the tests do not read the list they check.
var bootstrapSources = map[string]string{
	".walden/constitution.md":               "walden/constitution.md",
	".walden/lessons.md":                    "walden/lessons.md",
	".walden/.gitignore":                    "walden/gitignore",
	".github/pull_request_template.md":      "github/pull_request_template.md",
	".github/workflows/validate-walden.yml": "github/workflows/validate-walden.yml",
}

// customWorkflow is a generated workflow after a user adjusted it: an added
// trigger, a renamed install step and an added step. It installs v0.7.0.
const customWorkflow = `name: Validate Walden

on:
  workflow_dispatch:
  pull_request:
    paths:
      - ".walden/**"

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Check out repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - name: Install the walden CLI
        run: go install github.com/andrearaponi/walden/cmd/walden@v0.7.0

      - name: Print the version
        run: walden version

      - name: Validate Walden specs
        run: walden validate --all --json
`

// binaryDownloadWorkflow replaces the install step with a release download,
// so it names no install target.
const binaryDownloadWorkflow = `name: Validate Walden

on:
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Download walden
        run: |
          curl -fsSL -o walden https://github.com/andrearaponi/walden/releases/download/v0.14.5/walden-v0.14.5-linux-amd64
          chmod +x walden
      - name: Validate Walden specs
        run: ./walden validate --all --json
`

// initializedRoot returns a temporary repository with git metadata, after a
// first run of Init as the given version.
func initializedRoot(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	if _, err := Init(root, version); err != nil {
		t.Fatalf("first init: %v", err)
	}
	return root
}

func writeTarget(t *testing.T, root, target, content string) {
	t.Helper()
	path := filepath.Join(root, target)
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func readTarget(t *testing.T, root, target string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, target))
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	return string(content)
}

// restrictTarget sets a mode that differs from the one Init creates files
// with, so a run that sets the mode again is caught.
func restrictTarget(t *testing.T, root, target string) fs.FileMode {
	t.Helper()
	path := filepath.Join(root, target)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod %s: %v", target, err)
	}
	return targetMode(t, root, target)
}

func targetMode(t *testing.T, root, target string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(filepath.Join(root, target))
	if err != nil {
		t.Fatalf("stat %s: %v", target, err)
	}
	return info.Mode().Perm()
}

// templateContent reads a bootstrap template from the embedded file system
// and substitutes the version the way a release build would.
func templateContent(t *testing.T, target, version string) string {
	t.Helper()
	content, err := fs.ReadFile(templates.RepoFS(), bootstrapSources[target])
	if err != nil {
		t.Fatalf("read template of %s: %v", target, err)
	}
	return strings.ReplaceAll(string(content), "{{WALDEN_VERSION}}", version)
}

func userContent(target string) string {
	return "# Written by the user\n\nContent of " + target + " that no template holds.\n"
}

func TestInitKeepsExistingFiles(t *testing.T) {
	t.Run("edited after the first run", func(t *testing.T) {
		root := initializedRoot(t, "v0.7.1")
		modes := map[string]fs.FileMode{}
		for _, target := range keptTargets {
			writeTarget(t, root, target, userContent(target))
			modes[target] = restrictTarget(t, root, target)
		}

		report, err := Init(root, "v0.7.1")
		if err != nil {
			t.Fatalf("second init: %v", err)
		}

		for _, target := range keptTargets {
			if got := readTarget(t, root, target); got != userContent(target) {
				t.Errorf("%s was rewritten:\n got %q\nwant %q", target, got, userContent(target))
			}
			if got := targetMode(t, root, target); got != modes[target] {
				t.Errorf("%s changed mode: got %v, want %v", target, got, modes[target])
			}
			if !contains(report.SkippedFiles, target) {
				t.Errorf("%s is not reported as skipped: %#v", target, report.SkippedFiles)
			}
		}
		if len(report.UpdatedFiles) != 0 {
			t.Errorf("a run on edited files reports updated files: %#v", report.UpdatedFiles)
		}
		if len(report.CreatedFiles) != 0 {
			t.Errorf("a run on an initialized repository reports created files: %#v", report.CreatedFiles)
		}
	})

	t.Run("pull request template older than the first run", func(t *testing.T) {
		const target = ".github/pull_request_template.md"
		const own = "## What changed\n\nThe template this repository had before Walden.\n"
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".git"))
		writeTarget(t, root, target, own)

		report, err := Init(root, "v0.7.1")
		if err != nil {
			t.Fatalf("first init: %v", err)
		}

		if got := readTarget(t, root, target); got != own {
			t.Errorf("the first run rewrote %s:\n got %q\nwant %q", target, got, own)
		}
		if !contains(report.SkippedFiles, target) {
			t.Errorf("%s is not reported as skipped: %#v", target, report.SkippedFiles)
		}
		if contains(report.UpdatedFiles, target) || contains(report.CreatedFiles, target) {
			t.Errorf("%s is reported as written: updated %#v, created %#v", target, report.UpdatedFiles, report.CreatedFiles)
		}
	})
}

func TestInitRecreatesMissingFile(t *testing.T) {
	const version = "v0.7.1"
	// A workflow the user adjusted and that names the pin: a run keeps it.
	adjustedWorkflow := strings.Replace(customWorkflow, "cmd/walden@v0.7.0", "cmd/walden@"+version, 1)

	for deleted := range bootstrapSources {
		t.Run(deleted, func(t *testing.T) {
			root := initializedRoot(t, version)
			edited := map[string]string{}
			for _, target := range keptTargets {
				edited[target] = userContent(target)
			}
			edited[workflowTarget] = adjustedWorkflow
			delete(edited, deleted)
			for target, content := range edited {
				writeTarget(t, root, target, content)
			}
			if err := os.Remove(filepath.Join(root, deleted)); err != nil {
				t.Fatalf("delete %s: %v", deleted, err)
			}

			report, err := Init(root, version)
			if err != nil {
				t.Fatalf("second init: %v", err)
			}

			if got, want := readTarget(t, root, deleted), templateContent(t, deleted, version); got != want {
				t.Errorf("%s was not recreated from its template:\n got %q\nwant %q", deleted, got, want)
			}
			if !contains(report.CreatedFiles, deleted) {
				t.Errorf("%s is not reported as created: %#v", deleted, report.CreatedFiles)
			}
			if len(report.CreatedFiles) != 1 {
				t.Errorf("created files = %#v, want only %s", report.CreatedFiles, deleted)
			}
			for target, content := range edited {
				if got := readTarget(t, root, target); got != content {
					t.Errorf("%s, edited beside the deleted file, was rewritten:\n got %q\nwant %q", target, got, content)
				}
			}
		})
	}
}

func TestInitRefreshesPinInPlace(t *testing.T) {
	t.Run("adjusted workflow and a newer release", func(t *testing.T) {
		root := initializedRoot(t, "v0.7.0")
		writeTarget(t, root, workflowTarget, customWorkflow)
		mode := restrictTarget(t, root, workflowTarget)

		report, err := Init(root, "v0.7.1")
		if err != nil {
			t.Fatalf("second init: %v", err)
		}

		want := strings.Replace(customWorkflow, "cmd/walden@v0.7.0", "cmd/walden@v0.7.1", 1)
		if got := readTarget(t, root, workflowTarget); got != want {
			t.Errorf("the refresh changed more than the version, or not the version:\n got %q\nwant %q", got, want)
		}
		if got := targetMode(t, root, workflowTarget); got != mode {
			t.Errorf("the refresh changed the mode of the workflow: got %v, want %v", got, mode)
		}
		if !contains(report.UpdatedFiles, workflowTarget) {
			t.Errorf("the workflow is not reported as updated: %#v", report.UpdatedFiles)
		}
		if contains(report.SkippedFiles, workflowTarget) {
			t.Errorf("the refreshed workflow is reported as skipped: %#v", report.SkippedFiles)
		}
		if len(report.Warnings) != 0 {
			t.Errorf("a refreshing run carries warnings: %#v", report.Warnings)
		}
	})

	t.Run("workflow that installs the latest release", func(t *testing.T) {
		root := initializedRoot(t, "dev")
		if got := readTarget(t, root, workflowTarget); !strings.Contains(got, "cmd/walden@latest") {
			t.Fatalf("a source build did not generate a workflow that installs latest:\n%s", got)
		}

		report, err := Init(root, "v0.7.1")
		if err != nil {
			t.Fatalf("second init: %v", err)
		}

		if got, want := readTarget(t, root, workflowTarget), templateContent(t, workflowTarget, "v0.7.1"); got != want {
			t.Errorf("an unmodified workflow does not equal the template after the refresh:\n got %q\nwant %q", got, want)
		}
		if !contains(report.UpdatedFiles, workflowTarget) {
			t.Errorf("the workflow is not reported as updated: %#v", report.UpdatedFiles)
		}
	})

	t.Run("two install targets", func(t *testing.T) {
		const second = "\n  lint:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go install github.com/andrearaponi/walden/cmd/walden@latest\n"
		root := initializedRoot(t, "v0.7.0")
		writeTarget(t, root, workflowTarget, customWorkflow+second)

		if _, err := Init(root, "v0.7.1"); err != nil {
			t.Fatalf("second init: %v", err)
		}

		want := strings.Replace(customWorkflow, "cmd/walden@v0.7.0", "cmd/walden@v0.7.1", 1) +
			strings.Replace(second, "cmd/walden@latest", "cmd/walden@v0.7.1", 1)
		if got := readTarget(t, root, workflowTarget); got != want {
			t.Errorf("both versions were not replaced:\n got %q\nwant %q", got, want)
		}
	})
}

func TestInitKeepsWorkflowWithCurrentPin(t *testing.T) {
	const version = "v0.7.1"
	adjusted := strings.Replace(customWorkflow, "cmd/walden@v0.7.0", "cmd/walden@"+version, 1)
	root := initializedRoot(t, version)
	writeTarget(t, root, workflowTarget, adjusted)
	mode := restrictTarget(t, root, workflowTarget)

	report, err := Init(root, version)
	if err != nil {
		t.Fatalf("second init: %v", err)
	}

	if got := readTarget(t, root, workflowTarget); got != adjusted {
		t.Errorf("an adjusted workflow that names the pin was rewritten:\n got %q\nwant %q", got, adjusted)
	}
	if got := targetMode(t, root, workflowTarget); got != mode {
		t.Errorf("the run changed the mode of the workflow: got %v, want %v", got, mode)
	}
	if !contains(report.SkippedFiles, workflowTarget) {
		t.Errorf("the workflow is not reported as skipped: %#v", report.SkippedFiles)
	}
	if contains(report.UpdatedFiles, workflowTarget) {
		t.Errorf("the untouched workflow is reported as updated: %#v", report.UpdatedFiles)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("a run that finds the pin current carries warnings: %#v", report.Warnings)
	}
}

func TestInitKeepsWorkflowWithoutInstallTarget(t *testing.T) {
	legacy, err := os.ReadFile(filepath.Join("testdata", "validate-walden-v0.1.0.yml"))
	if err != nil {
		t.Fatalf("read the v0.1.0 workflow: %v", err)
	}

	cases := []struct {
		name     string
		workflow string
	}{
		{name: "written by v0.1.0", workflow: string(legacy)},
		{name: "install step replaced by a binary download", workflow: binaryDownloadWorkflow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.workflow, "cmd/walden@") {
				t.Fatalf("the fixture names an install target:\n%s", tc.workflow)
			}
			root := initializedRoot(t, "v0.7.1")
			writeTarget(t, root, workflowTarget, tc.workflow)

			report, err := Init(root, "v0.7.1")
			if err != nil {
				t.Fatalf("second init: %v", err)
			}

			if got := readTarget(t, root, workflowTarget); got != tc.workflow {
				t.Errorf("a workflow without an install target was rewritten:\n got %q\nwant %q", got, tc.workflow)
			}
			if !contains(report.SkippedFiles, workflowTarget) {
				t.Errorf("the workflow is not reported as skipped: %#v", report.SkippedFiles)
			}
			if contains(report.UpdatedFiles, workflowTarget) {
				t.Errorf("the untouched workflow is reported as updated: %#v", report.UpdatedFiles)
			}
			if len(report.Warnings) != 1 {
				t.Fatalf("warnings = %#v, want exactly one", report.Warnings)
			}
			for _, fragment := range []string{workflowTarget, "pin was not refreshed", "delete the file", "walden repo init", "regenerate"} {
				if !strings.Contains(report.Warnings[0], fragment) {
					t.Errorf("the warning does not mention %q: %s", fragment, report.Warnings[0])
				}
			}
		})
	}
}

func TestInitFailsBeforeWritingWhenWorkflowIsUnreadable(t *testing.T) {
	// A directory at the path of the workflow makes the read fail on every
	// platform, with no permission involved.
	blockWorkflow := func(t *testing.T, root string) {
		t.Helper()
		path := filepath.Join(root, workflowTarget)
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("clear the workflow path: %v", err)
		}
		mustMkdir(t, path)
	}

	t.Run("no bootstrap file is created", func(t *testing.T) {
		const constitution = ".walden/constitution.md"
		root := initializedRoot(t, "v0.7.1")
		if err := os.Remove(filepath.Join(root, constitution)); err != nil {
			t.Fatalf("delete the constitution: %v", err)
		}
		blockWorkflow(t, root)

		_, err := Init(root, "v0.7.1")
		if err == nil {
			t.Fatal("init succeeded with an unreadable workflow")
		}
		if !strings.Contains(err.Error(), workflowTarget) {
			t.Errorf("the error does not name the workflow: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, constitution)); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("the failed run created the constitution (stat error: %v)", statErr)
		}
	})

	t.Run("git is not initialized", func(t *testing.T) {
		root := t.TempDir()
		blockWorkflow(t, root)

		called := false
		previousRunner := gitInitRunner
		gitInitRunner = func(root string) error {
			called = true
			return os.MkdirAll(filepath.Join(root, ".git"), 0o755)
		}
		t.Cleanup(func() { gitInitRunner = previousRunner })

		if _, err := Init(root, "v0.7.1"); err == nil {
			t.Fatal("init succeeded with an unreadable workflow")
		}
		if called {
			t.Error("the failed run initialized git")
		}
		if _, statErr := os.Stat(filepath.Join(root, ".walden")); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("the failed run created .walden (stat error: %v)", statErr)
		}
	})
}
