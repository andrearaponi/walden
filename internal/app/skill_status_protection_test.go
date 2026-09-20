package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type skillStatusProtectedEntry struct {
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256,omitempty"`
	Target string `json:"target,omitempty"`
}

type skillStatusProtectedBaseline struct {
	Roots []string                             `json:"roots"`
	Files map[string]skillStatusProtectedEntry `json:"files"`
}

var skillStatusProtectedRoots = []string{
	"skill/walden", "cmd", "templates", "go.mod", "install.sh", "setup.sh", ".github/workflows",
	"internal/skilldist/stamp.go", "internal/skilldist/block.go", "internal/skilldist/install.go", "internal/skilldist/uninstall.go", "internal/skilldist/registry.go", "internal/skilldist/paths.go",
}

func checkSkillStatusProtectedInputs(root string, baseline skillStatusProtectedBaseline) error {
	if len(baseline.Roots) == 0 || len(baseline.Files) == 0 {
		return fmt.Errorf("empty protected baseline")
	}
	current := map[string]skillStatusProtectedEntry{}
	for _, name := range baseline.Roots {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe protected root %q", name)
		}
		err := filepath.WalkDir(filepath.Join(root, name), func(path string, item os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if item.IsDir() {
				return nil
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			entry := skillStatusProtectedEntry{Mode: uint32(info.Mode().Perm())}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				entry.Kind = "symlink"
				entry.Target, err = os.Readlink(path)
			case info.Mode().IsRegular():
				entry.Kind = "file"
				var data []byte
				data, err = os.ReadFile(path)
				if err == nil {
					entry.SHA256 = skillStatusHash(data)
				}
			default:
				return fmt.Errorf("unsupported protected file kind: %s", path)
			}
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			current[filepath.ToSlash(rel)] = entry
			return nil
		})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(current, baseline.Files) {
		return fmt.Errorf("protected input content/path/mode inventory changed")
	}
	return nil
}

func TestSkillStatusProtectedInputsUnchanged(t *testing.T) {
	for _, change := range []string{"none", "content", "missing", "added", "mode", "empty", "escape"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "fixture", "protected.go")
			skillStatusWriteFixture(t, path, []byte("ORIGINAL-PROTECTED-SENTINEL\n"))
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			entry := skillStatusProtectedEntry{Kind: "file", Mode: uint32(info.Mode().Perm()), SHA256: skillStatusFileHash(t, path)}
			baseline := skillStatusProtectedBaseline{Roots: []string{"fixture"}, Files: map[string]skillStatusProtectedEntry{"fixture/protected.go": entry}}
			switch change {
			case "content":
				skillStatusWriteFixture(t, path, []byte("CHANGED-PROTECTED-SENTINEL\n"))
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "added":
				skillStatusWriteFixture(t, filepath.Join(root, "fixture", "extra.go"), []byte("ADDED\n"))
			case "mode":
				// A changed expected mode exercises the comparison portably,
				// without assuming Windows implements Unix executable bits.
				entry.Mode ^= 0o100
				baseline.Files["fixture/protected.go"] = entry
			case "empty":
				baseline.Files = nil
			case "escape":
				baseline.Roots = []string{"../outside"}
			}
			if err := checkSkillStatusProtectedInputs(root, baseline); (err != nil) != (change != "none") {
				t.Fatalf("change=%s error=%v", change, err)
			}
		})
	}
	path := os.Getenv("WALDEN_SKILL_STATUS_BASELINE")
	if path == "" {
		return // Only synthetic contract controls run without an explicit baseline.
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringSourceRoot(t), path)
	}
	var baseline skillStatusProtectedBaseline
	if err := skillStatusDecode(path, &baseline); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(baseline.Roots, skillStatusProtectedRoots) {
		t.Fatal("unexpected or incomplete protected roots")
	}
	if err := checkSkillStatusProtectedInputs(authoringSourceRoot(t), baseline); err != nil {
		t.Fatal(err)
	}
}
