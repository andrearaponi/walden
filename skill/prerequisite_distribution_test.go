package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func bootstrapEnv(home string) []string {
	values := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "XDG_CACHE_HOME": filepath.Join(home, ".cache"),
		"CODEX_HOME": filepath.Join(home, ".codex"), "COPILOT_HOME": filepath.Join(home, ".copilot"), "OPENCODE_HOME": filepath.Join(home, ".opencode"),
		"DISABLE_TELEMETRY": "1", "DO_NOT_TRACK": "1", "CI": "1", "GIT_CONFIG_GLOBAL": filepath.Join(home, "empty-gitconfig"), "GIT_CONFIG_NOSYSTEM": "1",
	}
	var result []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, override := values[key]; !override {
			result = append(result, item)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func bootstrapProcess(t *testing.T, root, home, binary string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	cmd.Env = bootstrapEnv(home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestPrerequisiteCompiledDistribution(t *testing.T) {
	binary := prerequisiteBuild(t)
	root, home := t.TempDir(), t.TempDir()
	canonical := Content()
	if out := bootstrapProcess(t, root, home, binary, "skill", "show"); out != string(canonical) {
		t.Fatal("compiled guide differs from canonical source")
	}
	if out := bootstrapProcess(t, root, home, binary, "version", "--json"); !strings.Contains(out, "walden v0.10.4 (") {
		t.Fatal("incorrect kernel version")
	}
	const sentinel = "UNRELATED-CONTENT-BOOTSTRAP-7293\n"
	for _, path := range []string{filepath.Join(home, ".codex/AGENTS.md"), filepath.Join(root, "AGENTS.md")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, agent := range []string{"claude", "codex"} {
		bootstrapProcess(t, root, home, binary, "skill", "install", agent, "--json")
		bootstrapProcess(t, root, home, binary, "skill", "install", agent, "--project", "--json")
	}
	for _, path := range []string{filepath.Join(home, ".claude/skills/walden/SKILL.md"), filepath.Join(root, ".claude/skills/walden/SKILL.md")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := string(canonical)
		if !strings.HasSuffix(expected, "\n") {
			expected += "\n"
		}
		expected += "<!-- walden-skill-version: v0.10.4 -->\n"
		if string(data) != expected {
			t.Fatal("native installed guide/version mismatch")
		}
	}
	for _, path := range []string{filepath.Join(home, ".codex/AGENTS.md"), filepath.Join(root, "AGENTS.md")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), sentinel) || !bytes.Contains(data, canonical) {
			t.Fatal("Codex block replacement lost user content or canonical guide")
		}
	}
	var status struct {
		Result struct {
			Skills []struct {
				Agent, Scope, State, Version string
				Installed                    bool
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(bootstrapProcess(t, root, home, binary, "skill", "status", "--json")), &status); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, slot := range status.Result.Skills {
		if slot.Agent == "claude" || slot.Agent == "codex" {
			matched++
			if !slot.Installed || slot.State != "in-sync" || slot.Version != "v0.10.4" {
				t.Fatalf("bad native status %+v", slot)
			}
		}
	}
	if matched != 4 {
		t.Fatalf("missing native scope checks: %d", matched)
	}
}

func TestPrerequisiteSkillsCLIDistribution(t *testing.T) {
	tool := os.Getenv("WALDEN_SKILLS_CLI")
	if tool == "" {
		t.Skip("explicit prepared Skills CLI launcher not supplied")
	}
	if !filepath.IsAbs(tool) {
		tool = filepath.Join(authoringRoot(t), tool)
	}
	if info, err := os.Stat(tool); err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("required prepared tool missing/not executable: %s", tool)
	}
	repo := authoringRoot(t)
	export, workspace, home := t.TempDir(), t.TempDir(), t.TempDir()
	// Export only intended public skill resources, not local benchmark trees.
	for _, dir := range []string{"skill/walden", "skill/walden-history"} {
		err := filepath.WalkDir(filepath.Join(repo, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			rel, _ := filepath.Rel(repo, path)
			target := filepath.Join(export, rel)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o600)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	const sentinel = "UNRELATED-AGENT-FILE-BOOTSTRAP-451\n"
	for _, base := range []string{workspace, home} {
		if err := os.WriteFile(filepath.Join(base, "AGENTS.md"), []byte(sentinel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	list := bootstrapProcess(t, workspace, home, tool, "add", export, "--list")
	if !strings.Contains(list, "walden") {
		t.Fatalf("canonical guide not discovered: %s", list)
	}
	// Check the CLI's ordinary link installation and explicit copy mode.
	for _, copyMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("copy=%t", copyMode), func(t *testing.T) {
			project := t.TempDir()
			if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, base := range []string{".claude/skills", ".agents/skills"} {
				path := filepath.Join(project, base, "unrelated-sentinel", "SKILL.md")
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"add", export, "--skill", "walden", "--agent", "claude-code", "codex", "--yes"}
			if copyMode {
				args = append(args, "--copy")
			}
			bootstrapProcess(t, project, home, tool, args...)
			paths := []string{filepath.Join(project, ".claude/skills/walden/SKILL.md"), filepath.Join(project, ".agents/skills/walden/SKILL.md")}
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, Content()) {
					t.Fatalf("external guide mismatch at %s", path)
				}
			}
			if got, err := os.ReadFile(filepath.Join(project, "AGENTS.md")); err != nil || string(got) != sentinel {
				t.Fatal("unrelated agent instructions changed")
			}
			for _, base := range []string{".claude/skills", ".agents/skills"} {
				if _, err := os.Stat(filepath.Join(project, base, "walden-history")); !os.IsNotExist(err) {
					t.Fatal("unselected companion was installed")
				}
				data, err := os.ReadFile(filepath.Join(project, base, "unrelated-sentinel", "SKILL.md"))
				if err != nil || string(data) != sentinel {
					t.Fatal("unrelated skill was changed")
				}
			}
			info, err := os.Lstat(filepath.Join(project, ".claude/skills/walden"))
			if err != nil {
				t.Fatal(err)
			}
			if copyMode && info.Mode()&os.ModeSymlink != 0 {
				t.Fatal("explicit copy unexpectedly linked")
			}
			if !copyMode && info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("default installation did not use the expected canonical symlink")
			}
		})
	}
	for _, base := range []string{workspace, home} {
		data, err := os.ReadFile(filepath.Join(base, "AGENTS.md"))
		if err != nil || string(data) != sentinel {
			t.Fatal("unrelated home/workspace instructions changed")
		}
	}
}

type bootstrapSource struct {
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256,omitempty"`
	Target string `json:"target,omitempty"`
}
type bootstrapBaseline struct {
	Roots []string                   `json:"roots"`
	Files map[string]bootstrapSource `json:"files"`
}

func checkBootstrapSources(root string, baseline bootstrapBaseline) error {
	if len(baseline.Roots) == 0 || len(baseline.Files) == 0 {
		return fmt.Errorf("empty protected source baseline")
	}
	current := map[string]bootstrapSource{}
	for _, name := range baseline.Roots {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe protected root %q", name)
		}
		err := filepath.WalkDir(filepath.Join(root, name), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			item := bootstrapSource{Mode: uint32(info.Mode().Perm())}
			if info.Mode()&os.ModeSymlink != 0 {
				item.Kind = "symlink"
				item.Target, err = os.Readlink(path)
			} else if info.Mode().IsRegular() {
				item.Kind = "file"
				var data []byte
				data, err = os.ReadFile(path)
				if err == nil {
					item.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
				}
			} else {
				return fmt.Errorf("unsupported protected input kind: %s", path)
			}
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			current[filepath.ToSlash(relative)] = item
			return nil
		})
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(current, baseline.Files) {
		return fmt.Errorf("protected kernel/CLI/template/workflow inputs changed")
	}
	return nil
}

func TestPrerequisiteProtectedSourcesUnchanged(t *testing.T) {
	for _, change := range []string{"none", "content", "missing", "added", "mode"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "protected/file.go")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			baseline := bootstrapBaseline{Roots: []string{"protected"}, Files: map[string]bootstrapSource{"protected/file.go": {Kind: "file", Mode: 0o600, SHA256: prerequisiteHash(t, path)}}}
			switch change {
			case "content":
				os.WriteFile(path, []byte("changed"), 0o600)
			case "missing":
				os.Remove(path)
			case "added":
				os.WriteFile(filepath.Join(root, "protected/new.go"), []byte("added"), 0o600)
			case "mode":
				os.Chmod(path, 0o700)
			}
			err := checkBootstrapSources(root, baseline)
			if (err != nil) != (change != "none") {
				t.Fatalf("change=%s error=%v", change, err)
			}
		})
	}
	path := os.Getenv("WALDEN_BOOTSTRAP_BASELINE")
	if path == "" {
		return
	} // Synthetic positive/negative cases still ran above.
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	var baseline bootstrapBaseline
	prerequisiteReadJSON(t, path, &baseline)
	if !reflect.DeepEqual(baseline.Roots, []string{"internal", "cmd", "templates", "go.mod", ".github/workflows"}) {
		t.Fatal("unexpected protected roots")
	}
	if len(baseline.Files) == 0 {
		t.Fatal("empty protected-source baseline")
	}
	if err := checkBootstrapSources(authoringRoot(t), baseline); err != nil {
		t.Fatal(err)
	}
}
