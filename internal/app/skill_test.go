package app

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

// setSkillTestEnv points every skill-relevant environment variable at
// temporary directories so tests never touch the real user configuration.
func setSkillTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	t.Setenv("OPENCODE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

func TestSkillShowWritesVerbatimContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "show"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !bytes.Equal(stdout.Bytes(), skill.Content()) {
		t.Fatal("skill show must write the embedded skill verbatim, with no framing")
	}
}

func TestSkillShowJSONCarriesContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "show", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr.String())
	}

	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		Command       string `json:"command"`
		OK            bool   `json:"ok"`
		Result        struct {
			Content string `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	if envelope.Command != "skill-show" {
		t.Fatalf("expected command skill-show, got %s", envelope.Command)
	}
	if !envelope.OK {
		t.Fatal("expected ok envelope")
	}
	if envelope.Result.Content != string(skill.Content()) {
		t.Fatal("JSON content must equal the embedded skill")
	}
}

func TestSkillUnknownSubcommandFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"skill", "dance"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("unknown command")) {
		t.Fatalf("expected an unknown-command error, got: %s", stderr.String())
	}
}
