package evidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// belowTopLevel is where the tests put a root that is not the top level of
// its repository.
const belowTopLevel = "pkg/app"

// layoutRoot is an isolated repository with one commit, whose root lies at
// prefix below the top level, or is the top level for an empty prefix. Under
// the root it holds three files and a specification; with a prefix the
// repository also holds a file in another directory.
func layoutRoot(t *testing.T, prefix string) (top, root string) {
	t.Helper()
	top = isolatedRepo(t)
	root = filepath.Join(top, filepath.FromSlash(prefix))
	writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)
	writeTreeFile(t, root, "b.txt", "beta\n", 0o644)
	writeTreeFile(t, root, "dir/c.txt", "gamma\n", 0o644)
	writeTreeFile(t, root, ".walden/specs/demo/requirements.md", "spec\n", 0o644)
	if prefix != "" {
		writeTreeFile(t, top, "other/file.txt", "other\n", 0o644)
	}
	historyGit(t, top, "add", ".")
	historyGit(t, top, "commit", "-qm", "base")
	return top, root
}

func sortedKeys[V any](paths map[string]V) []string {
	keys := make([]string, 0, len(paths))
	for path := range paths {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}

// The same files give the same identity wherever the root lies in its
// repository, in every state of the working tree. The capture at the top
// level is the reference: TestCaptureTree holds it to git's own hash-object.
// The differing and untracked paths are written out here, and a state with a
// differing path must move the digest, so two captures that are equal and
// wrong do not pass.
func TestCaptureBelowTopLevel(t *testing.T) {
	ctx := context.Background()
	runner := shell.NewExecRunner()

	states := []struct {
		name      string
		unixOnly  bool
		change    func(t *testing.T, root string)
		differing []string
		absent    []string
		untracked []string
	}{
		{
			name:   "no uncommitted path",
			change: func(*testing.T, string) {},
		},
		{
			name: "one path modified",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
			},
			differing: []string{"a.txt"},
		},
		{
			name: "that change staged",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
				historyGit(t, root, "add", "a.txt")
			},
			differing: []string{"a.txt"},
		},
		{
			name: "one untracked path",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "new.txt", "new\n", 0o644)
			},
			differing: []string{"new.txt"},
			untracked: []string{"new.txt"},
		},
		{
			name: "one deleted path",
			change: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
					t.Fatal(err)
				}
			},
			differing: []string{"b.txt"},
			absent:    []string{"b.txt"},
		},
		{
			name: "one renamed path",
			change: func(t *testing.T, root string) {
				historyGit(t, root, "mv", "b.txt", "renamed.txt")
			},
			differing: []string{"b.txt", "renamed.txt"},
			absent:    []string{"b.txt"},
		},
		{
			name:     "one path whose mode changed",
			unixOnly: true,
			change: func(t *testing.T, root string) {
				if err := os.Chmod(filepath.Join(root, "a.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			differing: []string{"a.txt"},
		},
		{
			name: "a change under .walden only",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, ".walden/specs/demo/requirements.md", "changed\n", 0o644)
				writeTreeFile(t, root, ".walden/evidence/demo.json", "{}\n", 0o644)
			},
		},
		{
			name: "a path staged and then deleted",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "staged.txt", "staged\n", 0o644)
				historyGit(t, root, "add", "staged.txt")
				if err := os.Remove(filepath.Join(root, "staged.txt")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "an untracked path in a new directory",
			change: func(t *testing.T, root string) {
				writeTreeFile(t, root, "deep/er/file.txt", "deep\n", 0o644)
			},
			differing: []string{"deep/er/file.txt"},
			untracked: []string{"deep/er/file.txt"},
		},
		{
			name:     "an untracked symbolic link",
			unixOnly: true,
			change: func(t *testing.T, root string) {
				if err := os.Symlink("a.txt", filepath.Join(root, "link.txt")); err != nil {
					t.Fatal(err)
				}
			},
			differing: []string{"link.txt"},
			untracked: []string{"link.txt"},
		},
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			if state.unixOnly && runtime.GOOS == "windows" {
				t.Skip("needs file modes and symbolic links")
			}
			digests := map[string]string{}
			for _, layout := range []struct{ name, prefix string }{{"top level", ""}, {"below the top level", belowTopLevel}} {
				_, root := layoutRoot(t, layout.prefix)
				committed, ok := CaptureTree(ctx, runner, root)
				if !ok {
					t.Fatalf("%s: capture of the committed tree failed", layout.name)
				}
				state.change(t, root)
				capture, ok := CaptureTree(ctx, runner, root)
				if !ok {
					t.Fatalf("%s: capture failed", layout.name)
				}

				if got := sortedKeys(capture.Differing); !reflect.DeepEqual(got, append([]string{}, state.differing...)) {
					t.Errorf("%s: differing paths = %v, want %v", layout.name, got, state.differing)
				}
				for _, path := range state.absent {
					if entry, listed := capture.Differing[path]; !listed || entry != "" {
						t.Errorf("%s: %s differs as %q, want it absent", layout.name, path, entry)
					}
				}
				if got := sortedKeys(capture.Untracked); !reflect.DeepEqual(got, append([]string{}, state.untracked...)) {
					t.Errorf("%s: untracked paths = %v, want %v", layout.name, got, state.untracked)
				}
				moved := capture.Manifest.Digest() != committed.Manifest.Digest()
				if want := len(state.differing) > 0; moved != want {
					t.Errorf("%s: the digest moved = %t, want %t", layout.name, moved, want)
				}
				if capture.Commit == "" || capture.Commit != committed.Commit {
					t.Errorf("%s: the capture names commit %q, the committed one %q", layout.name, capture.Commit, committed.Commit)
				}
				plain, ok := CaptureManifest(ctx, runner, root)
				if !ok || plain.Digest() != capture.Manifest.Digest() {
					t.Errorf("%s: CaptureManifest and CaptureTree disagree (ok=%t)", layout.name, ok)
				}
				digests[layout.name] = capture.Manifest.Digest()
			}
			if digests["top level"] != digests["below the top level"] {
				t.Errorf("the same files give %s at the top level and %s below it", digests["top level"], digests["below the top level"])
			}
		})
	}
}

// A file outside the root is not part of the root's identity: neither a
// change to it nor the commit of that change moves the digest.
func TestCaptureKeepsPathsOutsideRootOut(t *testing.T) {
	ctx := context.Background()
	runner := shell.NewExecRunner()
	top, root := layoutRoot(t, belowTopLevel)

	before, ok := CaptureTree(ctx, runner, root)
	if !ok {
		t.Fatal("capture of the committed tree failed")
	}

	writeTreeFile(t, top, "other/file.txt", "changed\n", 0o644)
	writeTreeFile(t, top, "other/new.txt", "new\n", 0o644)
	modified, ok := CaptureTree(ctx, runner, root)
	if !ok {
		t.Fatal("capture with a change outside the root failed")
	}
	if modified.Manifest.Digest() != before.Manifest.Digest() || len(modified.Differing) != 0 || len(modified.Untracked) != 0 {
		t.Errorf("a change outside the root reached the capture: differing %v, untracked %v", modified.Differing, modified.Untracked)
	}
	if !modified.Committed() {
		t.Error("a change outside the root makes the root's tree read as uncommitted")
	}

	historyGit(t, top, "add", "other")
	historyGit(t, top, "commit", "-qm", "another directory")
	committed, ok := CaptureTree(ctx, runner, root)
	if !ok {
		t.Fatal("capture after the commit failed")
	}
	if committed.Manifest.Digest() != before.Manifest.Digest() {
		t.Error("the commit of a change outside the root moved the digest")
	}
	if committed.Commit == before.Commit {
		t.Error("the fixture did not move HEAD")
	}
	for path := range committed.Manifest {
		if filepath.ToSlash(path) == "other/file.txt" || filepath.ToSlash(path) == belowTopLevel+"/a.txt" {
			t.Errorf("the manifest holds %s: its paths must be relative to the root and stop at it", path)
		}
	}
}

// A capture never leaves out a path git reports. When it cannot place one
// under the root, because git does not say where the root lies or because
// the path is not under it, the capture fails and the identity is not
// available. A real git does not produce either condition with the pathspec
// the capture passes, so git is scripted here.
func TestCaptureFailsOnUnplacedPath(t *testing.T) {
	ctx := context.Background()
	listing := shell.Response{Stdout: lsTreeListing("100644 blob aaa\tmain.go")}

	refused := []struct {
		name string
		git  *fakeGit
	}{
		{"git fails the question", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: " M main.go\x00"}, "ls-tree": listing,
			"rev-parse": {ExitCode: 128, Stderr: "fatal: not a git repository"},
		}}},
		{"git cannot be started for the question", &fakeGit{
			responses: map[string]shell.Response{"status": {Stdout: " M main.go\x00"}, "ls-tree": listing},
			errors:    map[string]error{"rev-parse": errors.New("exec: git not found")},
		}},
		{"git answers the question with a status line", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: " M main.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: " M main.go\x00"},
		}}},
		{"a path in another directory", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: " M pkg/app/main.go\x00 M other/file.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "pkg/app/\n"},
		}}},
		{"a path at the top level", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: "?? main.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "pkg/app/\n"},
		}}},
		{"a rename whose source is outside the root", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: "R  pkg/app/main.go\x00other/old.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "pkg/app/\n"},
		}}},
		{"the root itself reported as one path", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: "?? pkg/app/\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "pkg/app/\n"},
		}}},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeTreeFile(t, root, "main.go", "changed\n", 0o644)
			if manifest, ok := CaptureManifest(ctx, c.git, root); ok {
				t.Errorf("CaptureManifest succeeded with %v", manifest)
			}
			if capture, ok := CaptureTree(ctx, c.git, root); ok {
				t.Errorf("CaptureTree succeeded with %v", capture.Manifest)
			}
			if identity, ok := Identity(ctx, c.git, root); ok || identity != "" {
				t.Errorf("Identity = %q, %t; want no identity", identity, ok)
			}
		})
	}

	placed := []struct {
		name string
		git  *fakeGit
	}{
		{"a path under a root below the top level", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: " M pkg/app/main.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "pkg/app/\n"},
		}}},
		{"a path under a root at the top level", &fakeGit{responses: map[string]shell.Response{
			"status": {Stdout: " M main.go\x00"}, "ls-tree": listing,
			"rev-parse": {Stdout: "\n"},
		}}},
	}
	for _, c := range placed {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeTreeFile(t, root, "main.go", "changed\n", 0o644)
			manifest, ok := CaptureManifest(ctx, c.git, root)
			if !ok {
				t.Fatal("the capture failed on a path it can place")
			}
			if want := (Manifest{"main.go": "100644:" + fakeBlob("changed\n")}); !reflect.DeepEqual(manifest, want) {
				t.Errorf("manifest = %v, want %v", manifest, want)
			}
		})
	}

	t.Run("a status without a path asks nothing", func(t *testing.T) {
		git := &fakeGit{
			responses: map[string]shell.Response{"status": {}, "ls-tree": listing},
			errors:    map[string]error{"rev-parse": errors.New("the question must not be asked")},
		}
		manifest, ok := CaptureManifest(ctx, git, t.TempDir())
		if !ok || !reflect.DeepEqual(manifest, Manifest{"main.go": "100644:aaa"}) {
			t.Errorf("the capture of a committed tree = %v, %t; want the listing of the commit", manifest, ok)
		}
	})
}

// argvRecorder runs real git and keeps the arguments of each call after
// `--no-optional-locks -C <root>`.
type argvRecorder struct{ calls [][]string }

func (r *argvRecorder) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	if len(args) > 3 {
		r.calls = append(r.calls, append([]string{}, args[3:]...))
	}
	return shell.NewExecRunner().Run(ctx, name, args...)
}

func (r *argvRecorder) subcommands() []string {
	subcommands := make([]string, 0, len(r.calls))
	for _, call := range r.calls {
		subcommands = append(subcommands, call[0])
	}
	return subcommands
}

// A capture on a tree with an uncommitted path adds one invocation, the
// question to git, and that invocation only reads. TestCaptureManifestInvocations
// holds the capture of a committed tree to the invocations it had before.
func TestCaptureInvocationsWithUncommittedPaths(t *testing.T) {
	ctx := context.Background()
	for _, layout := range []struct{ name, prefix string }{{"top level", ""}, {"below the top level", belowTopLevel}} {
		t.Run(layout.name, func(t *testing.T) {
			_, root := layoutRoot(t, layout.prefix)
			writeTreeFile(t, root, "a.txt", "changed\n", 0o644)

			plain := &argvRecorder{}
			if _, ok := CaptureManifest(ctx, plain, root); !ok {
				t.Fatal("CaptureManifest failed")
			}
			if want := []string{"status", "rev-parse", "ls-tree", "hash-object"}; !reflect.DeepEqual(plain.subcommands(), want) {
				t.Errorf("CaptureManifest ran %v, want %v", plain.subcommands(), want)
			}
			if len(plain.calls) > 1 && !reflect.DeepEqual(plain.calls[1], []string{"rev-parse", "--show-prefix"}) {
				t.Errorf("the added invocation is %v, want rev-parse --show-prefix", plain.calls[1])
			}

			withFacts := &argvRecorder{}
			if _, ok := CaptureTree(ctx, withFacts, root); !ok {
				t.Fatal("CaptureTree failed")
			}
			if want := []string{"rev-parse", "status", "rev-parse", "ls-tree", "hash-object"}; !reflect.DeepEqual(withFacts.subcommands(), want) {
				t.Errorf("CaptureTree ran %v, want %v", withFacts.subcommands(), want)
			}
		})
	}
}
