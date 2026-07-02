package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillStatusJSONFieldNames(t *testing.T) {
	status := SkillStatus{
		Agent:     "claude",
		Scope:     "user",
		Path:      "/home/u/.claude/skills/walden/SKILL.md",
		Installed: true,
		State:     "in-sync",
		Version:   "v1.0.0",
	}
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"agent", "scope", "path", "installed", "state", "version"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("expected JSON key %q, got %s", key, raw)
		}
	}
}

func TestResultSkillsAndContentAreAdditive(t *testing.T) {
	// Empty skills/content must not appear in the envelope at all, so
	// existing consumers see an unchanged shape.
	raw, err := json.Marshal(Result{Summary: "s"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"\"skills\"", "\"content\""} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("empty %s must be omitted from the envelope: %s", forbidden, raw)
		}
	}

	// Pre-existing envelope keys must remain present and unrenamed.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"summary", "changed_files", "skipped_files", "warnings", "exit_code"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("existing envelope key %q must remain, got %s", key, raw)
		}
	}

	withSkills, err := json.Marshal(Result{Skills: []SkillStatus{{Agent: "claude"}}, Content: "body"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(withSkills, []byte("\"skills\"")) || !bytes.Contains(withSkills, []byte("\"content\"")) {
		t.Fatalf("populated skills/content must serialize: %s", withSkills)
	}
}

func TestPrintTextRendersSkillsBlock(t *testing.T) {
	var buf bytes.Buffer
	PrintText(&buf, Result{
		Summary: "skill status",
		Skills: []SkillStatus{
			{Agent: "claude", Scope: "user", Path: "/p/SKILL.md", Installed: true, State: "in-sync", Version: "v1"},
			{Agent: "copilot", Scope: "user", Path: "/q/SKILL.md", Installed: false, State: "not-installed"},
		},
	})
	text := buf.String()
	if !strings.Contains(text, "Skills:") {
		t.Fatalf("expected a Skills block, got:\n%s", text)
	}
	if !strings.Contains(text, "claude (user): in-sync version=v1 path=/p/SKILL.md") {
		t.Fatalf("unexpected installed skill line:\n%s", text)
	}
	if !strings.Contains(text, "copilot (user): not-installed") {
		t.Fatalf("unexpected not-installed skill line:\n%s", text)
	}
	if strings.Contains(text, "path=/q/SKILL.md") {
		t.Fatalf("not-installed slots must not print a path:\n%s", text)
	}
}
