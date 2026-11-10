package evidence

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// isolatedRepo builds a temporary git repository that reads no global or
// system configuration, so the machine's commit signing, hooks, file-system
// monitor and line-ending conversion cannot reach the test. It sets process
// environment, which the git started by the code under test inherits too, so
// tests that use it are not parallel.
func isolatedRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	historyGit(t, root, "init", "-q", "-b", "main")
	historyGit(t, root, "config", "user.email", "fixture@walden.test")
	historyGit(t, root, "config", "user.name", "Fixture")
	return root
}

func writeTreeFile(t *testing.T, root, path, content string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// committedRepo is an isolated repository holding one commit of four files.
func committedRepo(t *testing.T) (root, head string) {
	t.Helper()
	root = isolatedRepo(t)
	writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)
	writeTreeFile(t, root, "b.txt", "beta\n", 0o644)
	writeTreeFile(t, root, "dir/c.txt", "gamma\n", 0o644)
	writeTreeFile(t, root, ".gitignore", "*.log\n", 0o644)
	historyGit(t, root, "add", ".")
	historyGit(t, root, "commit", "-qm", "base")
	return root, historyGit(t, root, "rev-parse", "HEAD")
}

// blobOf asks git itself for the blob id of a file: the oracle of a content
// identity is git, not the capture under test.
func blobOf(t *testing.T, root, path string) string {
	t.Helper()
	return historyGit(t, root, "hash-object", "--", path)
}

// blobOfText is the blob id git gives to a string, as it stores a symbolic
// link's target.
func blobOfText(t *testing.T, root, text string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(text)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git hash-object --stdin: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func TestCaptureTree(t *testing.T) {
	ctx := context.Background()
	runner := shell.NewExecRunner()

	cases := []struct {
		name        string
		unixOnly    bool
		change      func(t *testing.T, root string)
		differing   func(t *testing.T, root string) map[string]string
		untracked   []string
		notDiffered []string
	}{
		{
			name:      "clean",
			change:    func(*testing.T, string) {},
			differing: func(*testing.T, string) map[string]string { return map[string]string{} },
		},
		{
			name: "modified",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"a.txt": "100644:" + blobOf(t, root, "a.txt")}
			},
		},
		{
			name: "untracked",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "new.txt", "new\n", 0o644)
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"new.txt": "100644:" + blobOf(t, root, "new.txt")}
			},
			untracked: []string{"new.txt"},
		},
		{
			name: "deleted",
			change: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
					t.Fatal(err)
				}
			},
			differing: func(*testing.T, string) map[string]string { return map[string]string{"b.txt": ""} },
		},
		{
			name: "staged",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "dir/c.txt", "staged\n", 0o644)
				historyGit(t, root, "add", "dir/c.txt")
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"dir/c.txt": "100644:" + blobOf(t, root, "dir/c.txt")}
			},
		},
		{
			name: "renamed",
			change: func(t *testing.T, root string) {
				if err := os.Rename(filepath.Join(root, "a.txt"), filepath.Join(root, "z.txt")); err != nil {
					t.Fatal(err)
				}
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"a.txt": "", "z.txt": "100644:" + blobOf(t, root, "z.txt")}
			},
			untracked: []string{"z.txt"},
		},
		{
			// A rename staged with git mv: the source left the tree and the
			// target is in the index, so neither is untracked.
			name: "staged-rename",
			change: func(t *testing.T, root string) {
				historyGit(t, root, "mv", "a.txt", "z.txt")
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"a.txt": "", "z.txt": "100644:" + blobOf(t, root, "z.txt")}
			},
		},
		{
			// The same rename reported in the working tree, after git add -N
			// on the new path: the source is a field of the entry, not an
			// entry of its own.
			name: "renamed-intent-to-add",
			change: func(t *testing.T, root string) {
				if err := os.Rename(filepath.Join(root, "a.txt"), filepath.Join(root, "z.txt")); err != nil {
					t.Fatal(err)
				}
				historyGit(t, root, "add", "-N", "z.txt")
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"a.txt": "", "z.txt": "100644:" + blobOf(t, root, "z.txt")}
			},
		},
		{
			name:     "mode-changed",
			unixOnly: true,
			change: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "a.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"a.txt": "100755:" + blobOf(t, root, "a.txt")}
			},
		},
		{
			name:     "symlink",
			unixOnly: true,
			change: func(t *testing.T, root string) {
				if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			},
			differing: func(t *testing.T, root string) map[string]string {
				return map[string]string{"link": "120000:" + blobOfText(t, root, "a.txt")}
			},
			untracked: []string{"link"},
		},
		{
			name: "ignored-and-walden",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "debug.log", "noise\n", 0o644)
				writeTreeFile(t, root, ".walden/evidence/f.json", "{}\n", 0o644)
			},
			differing:   func(*testing.T, string) map[string]string { return map[string]string{} },
			notDiffered: []string{"debug.log", ".walden/evidence/f.json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unixOnly && runtime.GOOS == "windows" {
				t.Skip("mode bits and symbolic links are not portable to Windows")
			}
			root, head := committedRepo(t)
			tc.change(t, root)

			capture, ok := CaptureTree(ctx, runner, root)
			if !ok {
				t.Fatal("CaptureTree reported no usable capture")
			}
			if capture.Commit != head {
				t.Fatalf("commit = %q, want HEAD %q", capture.Commit, head)
			}
			if tc.differing != nil {
				want := tc.differing(t, root)
				if len(capture.Differing) != len(want) || (len(want) > 0 && !reflect.DeepEqual(capture.Differing, want)) {
					t.Fatalf("differing = %v, want %v", capture.Differing, want)
				}
				if len(capture.Untracked) != len(tc.untracked) {
					t.Fatalf("untracked = %v, want %v", capture.Untracked, tc.untracked)
				}
			}
			for _, path := range tc.notDiffered {
				if _, listed := capture.Differing[path]; listed {
					t.Fatalf("%s is outside the identity and must not be listed", path)
				}
			}
			for _, path := range tc.untracked {
				if !capture.Untracked[path] {
					t.Fatalf("%s not marked untracked: %v", path, capture.Untracked)
				}
			}

			// The same manifest as the identity-only capture: the facts
			// explain the identity, they do not change it.
			plain, plainOK := CaptureManifest(ctx, runner, root)
			if !plainOK || plain.Digest() != capture.Manifest.Digest() {
				t.Fatal("CaptureTree and CaptureManifest disagree on the manifest")
			}

			// The commit and the differing paths must rebuild the manifest
			// they were captured with, whatever layer each path sits in.
			listing := historyGit(t, root, "ls-tree", "-r", capture.Commit)
			if rebuilt := RebuildManifest(listing, capture.Differing); rebuilt.Digest() != capture.Manifest.Digest() {
				t.Fatalf("rebuilt manifest differs from the captured one:\nrebuilt %v\ncaptured %v", rebuilt, capture.Manifest)
			}
		})
	}

	t.Run("no-commit", func(t *testing.T) {
		root := isolatedRepo(t)
		writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)

		capture, ok := CaptureTree(ctx, runner, root)
		if !ok {
			t.Fatal("a repository without a commit still has a manifest")
		}
		if capture.Commit != "" || capture.Differing != nil {
			t.Fatalf("no commit to name, got commit %q and differing %v", capture.Commit, capture.Differing)
		}
		if _, exists := capture.Manifest["a.txt"]; !exists {
			t.Fatalf("manifest lacks the only file: %v", capture.Manifest)
		}
		if facts := capture.Facts(); facts != nil {
			t.Fatalf("no tree facts without a commit, got %+v", facts)
		}
	})

	t.Run("unscripted-rev-parse", func(t *testing.T) {
		// The scripted runners of the existing tests answer by subcommand
		// and know nothing of rev-parse: the capture must proceed as the
		// identity-only one does and yield no facts.
		for name, answer := range map[string]shell.Response{
			"empty":        {ExitCode: 0},
			"status-shape": {ExitCode: 0, Stdout: " M main.go\x00"},
			"failure":      {ExitCode: 128, Stderr: "fatal: needed a single revision"},
		} {
			t.Run(name, func(t *testing.T) {
				script := map[string]shell.Response{
					"status":    {ExitCode: 0},
					"ls-tree":   {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tmain.go")},
					"rev-parse": answer,
				}
				capture, ok := CaptureTree(ctx, &fakeGit{responses: script}, t.TempDir())
				if !ok {
					t.Fatal("CaptureTree failed under a scripted runner")
				}
				if capture.Commit != "" || capture.Differing != nil || capture.Facts() != nil {
					t.Fatalf("an answer that is not an object id must yield no facts: %+v", capture)
				}
				plain, _ := CaptureManifest(ctx, &fakeGit{responses: script}, t.TempDir())
				if plain.Digest() != capture.Manifest.Digest() {
					t.Fatal("the manifest changed with the fallback")
				}
			})
		}
	})

	t.Run("recording-limit", func(t *testing.T) {
		paths := map[string]string{}
		for i := 0; i < RecordingLimit; i++ {
			paths[filepath.Join("gen", strings.Repeat("x", i+1))] = "100644:aaa"
		}
		atLimit := Capture{Commit: strings.Repeat("a", 40), Differing: paths}.Facts()
		if atLimit == nil || len(atLimit.Paths) != RecordingLimit || atLimit.PathsOmitted != 0 {
			t.Fatalf("at the limit every path is kept, got %+v", atLimit)
		}
		paths["one-more"] = "100644:bbb"
		over := Capture{Commit: strings.Repeat("a", 40), Differing: paths}.Facts()
		if over == nil || over.Paths != nil || over.PathsOmitted != RecordingLimit+1 {
			t.Fatalf("over the limit only the count is kept, got %+v", over)
		}
	})
}

// subcommandRecorder runs real git and keeps the subcommand of each call.
type subcommandRecorder struct{ subcommands []string }

func (r *subcommandRecorder) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	// Git prefixes every call with --no-optional-locks -C <root>.
	if len(args) > 3 {
		r.subcommands = append(r.subcommands, args[3])
	}
	return shell.NewExecRunner().Run(ctx, name, args...)
}

// CaptureManifest serves every command that only needs the identity. Its git
// invocations are pinned: adoption planning is held to one status and one
// listing, and the scripted runners of the tests answer by subcommand.
func TestCaptureManifestInvocations(t *testing.T) {
	ctx := context.Background()
	root, _ := committedRepo(t)

	plain := &subcommandRecorder{}
	if _, ok := CaptureManifest(ctx, plain, root); !ok {
		t.Fatal("CaptureManifest failed on a clean repository")
	}
	if want := []string{"status", "ls-tree"}; !reflect.DeepEqual(plain.subcommands, want) {
		t.Fatalf("CaptureManifest ran %v, want %v", plain.subcommands, want)
	}

	withFacts := &subcommandRecorder{}
	if _, ok := CaptureTree(ctx, withFacts, root); !ok {
		t.Fatal("CaptureTree failed on a clean repository")
	}
	if want := []string{"rev-parse", "status", "ls-tree"}; !reflect.DeepEqual(withFacts.subcommands, want) {
		t.Fatalf("CaptureTree ran %v, want %v: it adds one invocation, the commit", withFacts.subcommands, want)
	}
}
