package evidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// RootPrefix reads one line of git: empty at the top level, the path down to
// the root with its slash below it. Anything else is no answer.
func TestRootPrefix(t *testing.T) {
	ctx := context.Background()

	t.Run("scripted answers", func(t *testing.T) {
		cases := []struct {
			name   string
			git    *fakeGit
			prefix string
			known  bool
		}{
			{"the top level", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "\n"}}}, "", true},
			{"no output at all", &fakeGit{responses: map[string]shell.Response{"rev-parse": {}}}, "", true},
			{"one directory down", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "app/\n"}}}, "app/", true},
			{"two directories down", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "pkg/app/\n"}}}, "pkg/app/", true},
			{"a carriage return before the newline", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "pkg/app/\r\n"}}}, "pkg/app/", true},
			{"a space in the path", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "my dir/app/\n"}}}, "my dir/app/", true},
			{"a status line", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: " M main.go\x00"}}}, "", false},
			{"an untracked status line", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "?? generated.txt\x00"}}}, "", false},
			{"an object id", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: strings.Repeat("a", 40) + "\n"}}}, "", false},
			{"a path without its slash", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "pkg/app\n"}}}, "", false},
			{"two lines", &fakeGit{responses: map[string]shell.Response{"rev-parse": {Stdout: "pkg/\napp/\n"}}}, "", false},
			{"a failed command", &fakeGit{responses: map[string]shell.Response{"rev-parse": {ExitCode: 128, Stderr: "fatal: not a git repository"}}}, "", false},
			{"a failed command that printed a path", &fakeGit{responses: map[string]shell.Response{"rev-parse": {ExitCode: 128, Stdout: "pkg/app/\n"}}}, "", false},
			{"a runner that cannot start git", &fakeGit{errors: map[string]error{"rev-parse": errors.New("exec: git not found")}}, "", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				prefix, known := RootPrefix(ctx, c.git, t.TempDir())
				if prefix != c.prefix || known != c.known {
					t.Fatalf("RootPrefix = %q, %t; want %q, %t", prefix, known, c.prefix, c.known)
				}
			})
		}
	})

	t.Run("real git", func(t *testing.T) {
		top := isolatedRepo(t)
		for _, c := range []struct{ dir, prefix string }{
			{"", ""},
			{"app", "app/"},
			{"pkg/app", "pkg/app/"},
			{"my dir/app", "my dir/app/"},
		} {
			root := filepath.Join(top, filepath.FromSlash(c.dir))
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			prefix, known := RootPrefix(ctx, shell.NewExecRunner(), root)
			if !known || prefix != c.prefix {
				t.Errorf("in %q: RootPrefix = %q, %t; want %q, true", c.dir, prefix, known, c.prefix)
			}
		}
		// A directory that git is told not to look above is in no repository.
		outside := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
		if prefix, known := RootPrefix(ctx, shell.NewExecRunner(), outside); known {
			t.Errorf("outside any repository: RootPrefix = %q, true; want no answer", prefix)
		}
	})
}

// placeStatus turns the paths git reports from the top level into paths from
// the root, and refuses an entry it cannot place under the root.
func TestPlaceStatus(t *testing.T) {
	cases := []struct {
		name    string
		entries []StatusEntry
		prefix  string
		want    []StatusEntry
		placed  bool
	}{
		{
			name:    "the top level keeps every path",
			entries: []StatusEntry{{Code: " M", Path: "main.go"}, {Code: "??", Path: "dir/new.go"}},
			prefix:  "",
			want:    []StatusEntry{{Code: " M", Path: "main.go"}, {Code: "??", Path: "dir/new.go"}},
			placed:  true,
		},
		{
			name:    "a root below the top level loses its prefix",
			entries: []StatusEntry{{Code: " M", Path: "pkg/app/main.go"}, {Code: "??", Path: "pkg/app/dir/new.go"}, {Code: " D", Path: "pkg/app/gone.go"}},
			prefix:  "pkg/app/",
			want:    []StatusEntry{{Code: " M", Path: "main.go"}, {Code: "??", Path: "dir/new.go"}, {Code: " D", Path: "gone.go"}},
			placed:  true,
		},
		{
			name:    "a rename is placed with its source",
			entries: []StatusEntry{{Code: "R ", Path: "pkg/app/new.go", Source: "pkg/app/old.go"}},
			prefix:  "pkg/app/",
			want:    []StatusEntry{{Code: "R ", Path: "new.go", Source: "old.go"}},
			placed:  true,
		},
		{
			name:    "a path under the root's .walden keeps that name",
			entries: []StatusEntry{{Code: " M", Path: "pkg/app/.walden/lessons.md"}},
			prefix:  "pkg/app/",
			want:    []StatusEntry{{Code: " M", Path: ".walden/lessons.md"}},
			placed:  true,
		},
		{
			name:    "no entry is nothing to place",
			entries: nil,
			prefix:  "pkg/app/",
			want:    []StatusEntry{},
			placed:  true,
		},
		{
			name:    "a path in another directory",
			entries: []StatusEntry{{Code: " M", Path: "pkg/app/main.go"}, {Code: " M", Path: "other/file.go"}},
			prefix:  "pkg/app/",
		},
		{
			name:    "a path at the top level",
			entries: []StatusEntry{{Code: " M", Path: "main.go"}},
			prefix:  "pkg/app/",
		},
		{
			name:    "a path that only begins like the prefix",
			entries: []StatusEntry{{Code: " M", Path: "pkg/application/main.go"}},
			prefix:  "pkg/app/",
		},
		{
			name:    "a rename whose source is outside the root",
			entries: []StatusEntry{{Code: "R ", Path: "pkg/app/new.go", Source: "other/old.go"}},
			prefix:  "pkg/app/",
		},
		{
			name:    "the root itself",
			entries: []StatusEntry{{Code: "??", Path: "pkg/app/"}},
			prefix:  "pkg/app/",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := append([]StatusEntry(nil), c.entries...)
			got, placed := placeStatus(c.entries, c.prefix)
			if placed != c.placed {
				t.Fatalf("placed = %t, want %t (entries %+v)", placed, c.placed, got)
			}
			if c.placed && !(len(got) == 0 && len(c.want) == 0) && !reflect.DeepEqual(got, c.want) {
				t.Fatalf("entries = %+v, want %+v", got, c.want)
			}
			if !c.placed && got != nil {
				t.Fatalf("a refusal still returned entries: %+v", got)
			}
			if !reflect.DeepEqual(c.entries, before) {
				t.Fatalf("the entries passed in were changed: %+v, were %+v", c.entries, before)
			}
		})
	}
}
