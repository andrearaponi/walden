package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// fakeBlob is the blob id fakeGit's hash-object gives a content.
func fakeBlob(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func seedRenameFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		name      string
		porcelain string
		want      []StatusEntry
		renamed   []bool
	}{
		{name: "empty", porcelain: "", want: []StatusEntry{}, renamed: []bool{}},
		{
			name:      "plain entries",
			porcelain: " M a.go\x00?? b c.go\x00",
			want:      []StatusEntry{{Code: " M", Path: "a.go"}, {Code: "??", Path: "b c.go"}},
			renamed:   []bool{false, false},
		},
		{
			name:      "staged rename",
			porcelain: "R  new.txt\x00old.txt\x00 M keep.txt\x00",
			want:      []StatusEntry{{Code: "R ", Path: "new.txt", Source: "old.txt"}, {Code: " M", Path: "keep.txt"}},
			renamed:   []bool{true, false},
		},
		{
			name:      "rename in the working tree",
			porcelain: " R new.txt\x00old.txt\x00",
			want:      []StatusEntry{{Code: " R", Path: "new.txt", Source: "old.txt"}},
			renamed:   []bool{true},
		},
		{
			name:      "copy",
			porcelain: "C  copy.txt\x00old.txt\x00",
			want:      []StatusEntry{{Code: "C ", Path: "copy.txt", Source: "old.txt"}},
			renamed:   []bool{false},
		},
		{
			// A source is consumed whatever its length: a one-letter path
			// is a source too, not a field to skip.
			name:      "short source",
			porcelain: "R  new.txt\x00a\x00 M b.go\x00",
			want:      []StatusEntry{{Code: "R ", Path: "new.txt", Source: "a"}, {Code: " M", Path: "b.go"}},
			renamed:   []bool{true, false},
		},
		{
			name:      "rename cut before its source",
			porcelain: "R  new.txt",
			want:      []StatusEntry{{Code: "R ", Path: "new.txt"}},
			renamed:   []bool{false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStatus(tc.porcelain)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("entries = %+v, want %+v", got, tc.want)
			}
			for index, entry := range got {
				if entry.Renamed() != tc.renamed[index] {
					t.Fatalf("entry %+v: Renamed() = %t, want %t", entry, entry.Renamed(), tc.renamed[index])
				}
			}
		})
	}
}

// The source of a rename is no longer in the working tree. Its entry comes
// from the commit's listing and must leave the manifest in every shape git
// reports a rename. A copy keeps its source, which is still there.
func TestIdentityRenameSourceLeavesTheManifest(t *testing.T) {
	listing := lsTreeListing(
		"100644 blob aaa\told.txt",
		"100644 blob bbb\tkeep.txt",
	)
	cases := []struct {
		name    string
		listing string
		status  string
		files   map[string]string
		want    Manifest
	}{
		{
			name:   "staged rename",
			status: "R  new.txt\x00old.txt\x00",
			files:  map[string]string{"new.txt": "content", "keep.txt": "kept"},
			want:   Manifest{"keep.txt": "100644:bbb", "new.txt": "100644:" + fakeBlob("content")},
		},
		{
			name:   "staged rename, then modified",
			status: "RM new.txt\x00old.txt\x00",
			files:  map[string]string{"new.txt": "edited", "keep.txt": "kept"},
			want:   Manifest{"keep.txt": "100644:bbb", "new.txt": "100644:" + fakeBlob("edited")},
		},
		{
			name:   "staged rename, then deleted",
			status: "RD new.txt\x00old.txt\x00",
			files:  map[string]string{"keep.txt": "kept"},
			want:   Manifest{"keep.txt": "100644:bbb"},
		},
		{
			// What `git add -N` on the new path reports: the rename sits in
			// the working-tree column.
			name:   "rename in the working tree",
			status: " R new.txt\x00old.txt\x00",
			files:  map[string]string{"new.txt": "content", "keep.txt": "kept"},
			want:   Manifest{"keep.txt": "100644:bbb", "new.txt": "100644:" + fakeBlob("content")},
		},
		{
			// Read as an entry of its own, the source field "ab/keep.txt"
			// would name keep.txt and replace its committed entry with the
			// content on disk.
			name: "source field is not an entry",
			listing: lsTreeListing(
				"100644 blob aaa\tab/keep.txt",
				"100644 blob bbb\tkeep.txt",
			),
			status: " R new.txt\x00ab/keep.txt\x00",
			files:  map[string]string{"new.txt": "content", "keep.txt": "kept"},
			want:   Manifest{"keep.txt": "100644:bbb", "new.txt": "100644:" + fakeBlob("content")},
		},
		{
			name:   "source created again",
			status: "R  new.txt\x00old.txt\x00?? old.txt\x00",
			files:  map[string]string{"new.txt": "content", "old.txt": "again", "keep.txt": "kept"},
			want: Manifest{
				"keep.txt": "100644:bbb",
				"new.txt":  "100644:" + fakeBlob("content"),
				"old.txt":  "100644:" + fakeBlob("again"),
			},
		},
		{
			name:   "source created again, listed first",
			status: "?? old.txt\x00R  new.txt\x00old.txt\x00",
			files:  map[string]string{"new.txt": "content", "old.txt": "again", "keep.txt": "kept"},
			want: Manifest{
				"keep.txt": "100644:bbb",
				"new.txt":  "100644:" + fakeBlob("content"),
				"old.txt":  "100644:" + fakeBlob("again"),
			},
		},
		{
			name:   "copy keeps its source",
			status: "C  copy.txt\x00old.txt\x00",
			files:  map[string]string{"copy.txt": "content", "old.txt": "content", "keep.txt": "kept"},
			want: Manifest{
				"keep.txt": "100644:bbb",
				"old.txt":  "100644:aaa",
				"copy.txt": "100644:" + fakeBlob("content"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			seedRenameFiles(t, root, tc.files)
			committed := tc.listing
			if committed == "" {
				committed = listing
			}
			git := &fakeGit{responses: map[string]shell.Response{
				"status":  {ExitCode: 0, Stdout: tc.status},
				"ls-tree": {ExitCode: 0, Stdout: committed},
			}}

			manifest, ok := CaptureManifest(context.Background(), git, root)
			if !ok {
				t.Fatal("manifest not captured")
			}
			if !reflect.DeepEqual(manifest, tc.want) {
				t.Fatalf("manifest = %v, want %v", manifest, tc.want)
			}
		})
	}
}

// Committing a staged rename changes no content, so it must not move the
// identity: the staged tree and the committed tree hold the same files.
func TestIdentitySurvivesTheRenameCommitTransition(t *testing.T) {
	root := t.TempDir()
	seedRenameFiles(t, root, map[string]string{"new.txt": "content"})
	blob := fakeBlob("content")

	staged := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "R  new.txt\x00old.txt\x00"},
		"ls-tree": {ExitCode: 0, Stdout: "100644 blob " + blob + "\told.txt\n"},
	}}
	committed := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: "100644 blob " + blob + "\tnew.txt\n"},
	}}

	before, ok := Identity(context.Background(), staged, root)
	if !ok {
		t.Fatal("staged identity not computed")
	}
	after, ok := Identity(context.Background(), committed, root)
	if !ok {
		t.Fatal("committed identity not computed")
	}
	if before != after {
		t.Fatalf("committing a staged rename moved the identity: %s vs %s", before, after)
	}
}

// While a rename is staged, a file created again at the old path is a change
// to the working tree even when it holds the content the commit had there.
func TestIdentitySeesARecreatedRenameSource(t *testing.T) {
	root := t.TempDir()
	seedRenameFiles(t, root, map[string]string{"new.txt": "content"})
	listing := "100644 blob " + fakeBlob("content") + "\told.txt\n"

	staged := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "R  new.txt\x00old.txt\x00"},
		"ls-tree": {ExitCode: 0, Stdout: listing},
	}}
	before, ok := Identity(context.Background(), staged, root)
	if !ok {
		t.Fatal("staged identity not computed")
	}

	seedRenameFiles(t, root, map[string]string{"old.txt": "content"})
	recreated := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "R  new.txt\x00old.txt\x00?? old.txt\x00"},
		"ls-tree": {ExitCode: 0, Stdout: listing},
	}}
	after, ok := Identity(context.Background(), recreated, root)
	if !ok {
		t.Fatal("identity not computed with the source created again")
	}
	if before == after {
		t.Fatal("a file created again at the old path of a staged rename did not move the identity")
	}
}

// renameRepo is a temporary git repository that reads no global or system
// configuration: a machine setting such as status.renames would change what
// git reports. It sets process environment, so its tests are not parallel.
func renameRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required by this acceptance fixture")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	historyGit(t, root, "init", "-q", "-b", "main")
	historyGit(t, root, "config", "user.email", "fixture@walden.test")
	historyGit(t, root, "config", "user.name", "Fixture")
	seedRenameFiles(t, root, map[string]string{
		"old.txt":  "same content\nline two\nline three\n",
		"keep.txt": "kept\n",
	})
	historyGit(t, root, "add", ".")
	historyGit(t, root, "commit", "-qm", "base")
	return root
}

// The same three facts against git itself, which decides what a rename looks
// like in its status output.
func TestIdentityOfARenameAgainstGit(t *testing.T) {
	identity := func(t *testing.T, root string) string {
		t.Helper()
		id, ok := Identity(context.Background(), shell.NewExecRunner(), root)
		if !ok {
			t.Fatal("identity not computed")
		}
		return id
	}
	manifest := func(t *testing.T, root string) Manifest {
		t.Helper()
		captured, ok := CaptureManifest(context.Background(), shell.NewExecRunner(), root)
		if !ok {
			t.Fatal("manifest not captured")
		}
		return captured
	}

	t.Run("staged rename, then commit", func(t *testing.T) {
		root := renameRepo(t)
		historyGit(t, root, "mv", "old.txt", "new.txt")

		captured := manifest(t, root)
		if _, held := captured["old.txt"]; held {
			t.Fatalf("the manifest still holds the old path of a staged rename: %v", captured)
		}
		if _, held := captured["new.txt"]; !held {
			t.Fatalf("the manifest lacks the new path of a staged rename: %v", captured)
		}

		staged := identity(t, root)
		historyGit(t, root, "commit", "-qm", "rename")
		if committed := identity(t, root); committed != staged {
			t.Fatalf("committing a staged rename moved the identity: %s vs %s", staged, committed)
		}
	})

	t.Run("source created again", func(t *testing.T) {
		root := renameRepo(t)
		historyGit(t, root, "mv", "old.txt", "new.txt")
		staged := identity(t, root)

		seedRenameFiles(t, root, map[string]string{"old.txt": "same content\nline two\nline three\n"})
		if recreated := identity(t, root); recreated == staged {
			t.Fatal("a file created again at the old path of a staged rename did not move the identity")
		}
	})

	t.Run("rename in the working tree", func(t *testing.T) {
		root := renameRepo(t)
		if err := os.Rename(filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")); err != nil {
			t.Fatal(err)
		}
		historyGit(t, root, "add", "-N", "new.txt")

		captured := manifest(t, root)
		want := map[string]bool{"keep.txt": true, "new.txt": true}
		if len(captured) != len(want) {
			t.Fatalf("manifest paths = %v, want keep.txt and new.txt", captured)
		}
		for path := range want {
			if _, held := captured[path]; !held {
				t.Fatalf("manifest paths = %v, want keep.txt and new.txt", captured)
			}
		}
	})
}
