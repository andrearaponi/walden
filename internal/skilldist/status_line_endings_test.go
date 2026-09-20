package skilldist

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

func accuracyCRLF(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}

func accuracyMixed(data []byte) []byte {
	lines := bytes.SplitAfter(data, []byte("\n"))
	var result []byte
	for i, line := range lines {
		if i%2 == 0 {
			line = accuracyCRLF(line)
		}
		result = append(result, line...)
	}
	return result
}

func accuracyShared(data []byte) []byte {
	result := []byte("UNRELATED-PREFIX-891\r\n" + blockBegin + "\r\n")
	result = append(result, data...)
	if !bytes.HasSuffix(result, []byte("\n")) {
		result = append(result, '\r', '\n')
	}
	return append(result, []byte(blockEnd+"\r\nUNRELATED-SUFFIX-734\n")...)
}

func TestSkillStatusLineEndingAccuracy(t *testing.T) {
	canonical := bytes.ReplaceAll(skill.Content(), []byte("\r\n"), []byte("\n"))
	stamp := Stamp(canonical, "v1.2.3")
	cases := []struct {
		name, state, version string
		data                 []byte
	}{
		{"LF unstamped", StateInSync, "", canonical},
		{"CRLF unstamped", StateInSync, "", accuracyCRLF(canonical)},
		{"mixed unstamped", StateInSync, "", accuracyMixed(canonical)},
		{"LF stamped", StateInSync, "v1.2.3", stamp},
		{"CRLF stamped", StateInSync, "v1.2.3", accuracyCRLF(stamp)},
		{"mixed stamped", StateInSync, "v1.2.3", accuracyMixed(stamp)},
		{"marker EOF", StateInSync, "v1.2.3", bytes.TrimSuffix(stamp, []byte("\n"))},
		{"CRLF body marker EOF", StateInSync, "v1.2.3", bytes.TrimSuffix(accuracyCRLF(stamp), []byte("\r\n"))},
		{"CRLF body LF marker", StateInSync, "v1.2.3", Stamp(accuracyCRLF(canonical), "v1.2.3")},
		{"trailing newlines", StateInSync, "", append(append([]byte(nil), accuracyCRLF(canonical)...), []byte("\r\n\n\r\n")...)},
		{"changed word LF", StateDrifted, "", bytes.Replace(canonical, []byte("Walden"), []byte("CHANGED-NAME-195"), 1)},
		{"changed word CRLF", StateDrifted, "", accuracyCRLF(bytes.Replace(canonical, []byte("Walden"), []byte("CHANGED-NAME-195"), 1))},
		{"added instruction", StateDrifted, "", append(append([]byte(nil), canonical...), []byte("ADDED-INSTRUCTION-246\n")...)},
		{"leading whitespace", StateDrifted, "", append([]byte(" "), canonical...)},
		{"interior whitespace CRLF", StateDrifted, "", accuracyCRLF(bytes.Replace(canonical, []byte("\n"), []byte(" \n"), 1))},
		{"BOM", StateDrifted, "", append([]byte{0xef, 0xbb, 0xbf}, canonical...)},
		{"lone CR", StateDrifted, "", bytes.Replace(canonical, []byte("\n"), []byte("\r"), 1)},
		{"malformed marker", StateDrifted, "", append(append([]byte(nil), canonical...), []byte("<!-- walden-skill-version: v1.2.3 --!>\n")...)},
		{"non-trailing marker", StateDrifted, "", append(append([]byte(nil), stamp...), []byte("AFTER-MARKER-CONTENT-726\n")...)},
	}
	for _, agent := range []string{"claude", "codex"} {
		for _, tc := range cases {
			t.Run(agent+"/"+tc.name, func(t *testing.T) {
				opts, home := testOptions(t)
				target := filepath.Join(home, ".claude", "skills", "walden", "SKILL.md")
				data := tc.data
				if agent == "codex" {
					target = filepath.Join(home, ".codex", "AGENTS.md")
					data = accuracyShared(data)
				}
				accuracyWrite(t, target, data)
				slots, warnings := Status(opts)
				accuracySlot(t, slots, agent, ScopeUser, tc.state, true, tc.version)
				if len(warnings) != 0 {
					t.Fatalf("readable single slot unexpectedly warned: %v", warnings)
				}
				after, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(after, data) {
					t.Fatalf("inspection rewrote installed bytes: %v", err)
				}
			})
		}
	}
	t.Run("embedded comparison view", func(t *testing.T) {
		if !bytes.Equal(normalizeBody(canonical), normalizeBody(accuracyCRLF(canonical))) {
			t.Fatal("embedded and installed comparison views must normalize identically")
		}
	})
	t.Run("dual scope comparison", func(t *testing.T) {
		opts, home := testOptions(t)
		accuracyWrite(t, filepath.Join(home, ".claude", "skills", "walden", "SKILL.md"), accuracyCRLF(stamp))
		project := filepath.Join(opts.WorkDir, ".claude", "skills", "walden", "SKILL.md")
		accuracyWrite(t, project, canonical)
		_, warnings := Status(opts)
		if len(warnings) != 0 {
			t.Fatalf("EOL or stamp alone must not cause scope divergence: %v", warnings)
		}
		accuracyWrite(t, project, append(append([]byte(nil), canonical...), []byte("SCOPE-DIFFERENCE-562\n")...))
		_, warnings = Status(opts)
		if !accuracyDiverges(warnings) {
			t.Fatal("substantive scope divergence hidden by normalization")
		}
	})
}

func accuracySnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for i, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			value := fmt.Sprintf("%v/%d", info.Mode(), info.ModTime().UnixNano())
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				value += "/" + target
			} else if info.Mode().IsRegular() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value += fmt.Sprintf("/%x", sha256.Sum256(data))
			}
			result[fmt.Sprintf("%d/%s", i, filepath.ToSlash(rel))] = value
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestSkillStatusInspectionPreservesBytes(t *testing.T) {
	opts, home := testOptions(t)
	canonical := bytes.ReplaceAll(skill.Content(), []byte("\r\n"), []byte("\n"))
	for _, agent := range Agents() {
		for _, scope := range []Scope{ScopeUser, ScopeProject} {
			target, err := resolveTarget(agent, scope, opts)
			if err != nil {
				continue // Only the two registry-declared unsupported project scopes.
			}
			data := accuracyCRLF(Stamp(canonical, "v1.2.3"))
			if agent.Kind == KindBlock {
				data = accuracyShared(data)
			}
			accuracyWrite(t, target, data)
		}
	}
	accuracyWrite(t, filepath.Join(home, ".agents", "skills", "walden", "SKILL.md"), []byte("EXTERNAL-MANAGER-CONTENT-499\r\n"))
	accuracyWrite(t, filepath.Join(home, ".agents", ".skill-lock.json"), []byte("{\"unchanged\":true}\r\n"))
	accuracyWrite(t, filepath.Join(home, ".gitconfig"), []byte("[core]\r\n  autocrlf = true\r\n"))
	accuracyWrite(t, filepath.Join(home, ".profile"), []byte("PROFILE-CONTENT-175\r\n"))
	before := accuracySnapshot(t, home, opts.WorkDir)
	slots, warnings := Status(opts)
	if len(slots) != 6 || len(warnings) != 0 {
		t.Fatalf("unexpected inventory/warnings: %+v %v", slots, warnings)
	}
	for _, slot := range slots {
		if !slot.Installed || slot.State != StateInSync || slot.Version != "v1.2.3" {
			t.Errorf("must recognize, not rewrite, the CRLF installation: %+v", slot)
		}
	}
	if after := accuracySnapshot(t, home, opts.WorkDir); !reflect.DeepEqual(before, after) {
		t.Fatal("status changed file bytes, kind, mode, modification time or link target")
	}
	// These shared helpers retain their raw-byte contract; normalization belongs
	// only to status's private comparison view, never to a writer's input.
	raw := []byte("RAW-ROUNDTRIP-SENTINEL-192\r\n")
	stamped := Stamp(raw, "v4.5.6")
	body, version := Strip(stamped)
	if !bytes.Equal(body, raw) || version != "v4.5.6" || !bytes.HasPrefix(stamped, raw) {
		t.Fatal("raw stamp/strip semantics changed")
	}
	if body, version := Strip(raw); !bytes.Equal(body, raw) || version != "" {
		t.Fatal("legacy raw bytes changed")
	}
	if !strings.Contains(string(stamped), "\r\n") {
		t.Fatal("round-trip control did not retain CRLF")
	}
}
