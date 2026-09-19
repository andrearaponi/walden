package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skill status/install must resolve the version the same way `walden version`
// does: ldflags first, then a release module version from build info, else dev.
func TestSkillCommandsUseEffectiveVersion(t *testing.T) {
	cases := []struct {
		name      string
		ldflags   string
		buildInfo string
		want      string
	}{
		{"release build", "v0.10.4", "(devel)", "v0.10.4"},
		{"go install", "dev", "v0.10.4", "v0.10.4"},
		{"source build", "dev", "(devel)", "dev"},
		{"pseudo-version", "dev", "v0.10.4-0.20260919120000-abcdef123456", "dev"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreVersion, restoreSeam := Version, buildInfoVersion
			t.Cleanup(func() { Version, buildInfoVersion = restoreVersion, restoreSeam })
			Version = tc.ldflags
			buildInfoVersion = func() string { return tc.buildInfo }
			home := setSkillTestEnv(t)
			t.Chdir(t.TempDir())

			var out bytes.Buffer
			if code := Run([]string{"skill", "install", "claude", "--json"}, &out, &out); code != 0 {
				t.Fatalf("install failed:\n%s", out.String())
			}
			installed, err := os.ReadFile(filepath.Join(home, ".claude", "skills", "walden", "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(installed), "<!-- walden-skill-version: "+tc.want+" -->\n") {
				t.Errorf("marker: want version %q, got tail %q", tc.want, tail(string(installed)))
			}

			var stdout, stderr bytes.Buffer
			if code := Run([]string{"skill", "status", "--json"}, &stdout, &stderr); code != 0 {
				t.Fatalf("status failed: %s", stderr.String())
			}
			var envelope struct {
				Result struct {
					Summary string `json:"summary"`
					Skills  []struct {
						Agent, Scope, Version string
					} `json:"skills"`
				} `json:"result"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if want := "skill status against embedded version " + tc.want; envelope.Result.Summary != want {
				t.Errorf("summary: want %q, got %q", want, envelope.Result.Summary)
			}
			for _, slot := range envelope.Result.Skills {
				if slot.Agent == "claude" && slot.Scope == "user" && slot.Version != tc.want {
					t.Errorf("status slot version: want %q, got %q", tc.want, slot.Version)
				}
			}
		})
	}
}

func tail(s string) string {
	if len(s) > 60 {
		return s[len(s)-60:]
	}
	return s
}
