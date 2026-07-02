package skilldist

import (
	"testing"
)

func TestPathsUserTargets(t *testing.T) {
	cases := []struct {
		name  string
		agent string
		env   Env
		want  string
	}{
		{
			name:  "claude under home",
			agent: "claude",
			env:   Env{Home: "/home/u"},
			want:  "/home/u/.claude/skills/walden/SKILL.md",
		},
		{
			name:  "codex default home",
			agent: "codex",
			env:   Env{Home: "/home/u"},
			want:  "/home/u/.codex/AGENTS.md",
		},
		{
			name:  "codex honors CODEX_HOME",
			agent: "codex",
			env:   Env{Home: "/home/u", CodexHome: "/custom/codex"},
			want:  "/custom/codex/AGENTS.md",
		},
		{
			name:  "copilot default home",
			agent: "copilot",
			env:   Env{Home: "/home/u"},
			want:  "/home/u/.copilot/skills/walden/SKILL.md",
		},
		{
			name:  "copilot honors COPILOT_HOME",
			agent: "copilot",
			env:   Env{Home: "/home/u", CopilotHome: "/custom/copilot"},
			want:  "/custom/copilot/skills/walden/SKILL.md",
		},
		{
			name:  "opencode default home",
			agent: "opencode",
			env:   Env{Home: "/home/u"},
			want:  "/home/u/.config/opencode/skills/walden/SKILL.md",
		},
		{
			name:  "opencode honors XDG_CONFIG_HOME",
			agent: "opencode",
			env:   Env{Home: "/home/u", XDGConfigHome: "/xdg"},
			want:  "/xdg/opencode/skills/walden/SKILL.md",
		},
		{
			name:  "opencode honors OPENCODE_HOME over XDG",
			agent: "opencode",
			env:   Env{Home: "/home/u", XDGConfigHome: "/xdg", OpencodeHome: "/custom/opencode"},
			want:  "/custom/opencode/skills/walden/SKILL.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent, err := Lookup(tc.agent)
			if err != nil {
				t.Fatalf("Lookup(%s): %v", tc.agent, err)
			}
			got, err := agent.UserTarget(tc.env)
			if err != nil {
				t.Fatalf("UserTarget: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestPathsUserTargetsRequireHome(t *testing.T) {
	for _, name := range AgentNames() {
		agent, err := Lookup(name)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", name, err)
		}
		if _, err := agent.UserTarget(Env{}); err == nil {
			t.Fatalf("%s: expected error for unresolvable home", name)
		}
	}
}

func TestPathsCustomHomesSkipHomeRequirement(t *testing.T) {
	cases := []struct {
		agent string
		env   Env
	}{
		{"codex", Env{CodexHome: "/custom/codex"}},
		{"copilot", Env{CopilotHome: "/custom/copilot"}},
		{"opencode", Env{OpencodeHome: "/custom/opencode"}},
		{"opencode", Env{XDGConfigHome: "/xdg"}},
	}
	for _, tc := range cases {
		agent, err := Lookup(tc.agent)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", tc.agent, err)
		}
		if _, err := agent.UserTarget(tc.env); err != nil {
			t.Fatalf("%s: expected resolution without home, got %v", tc.agent, err)
		}
	}
}

func TestPathsLegacyUserFiles(t *testing.T) {
	claude, err := Lookup("claude")
	if err != nil {
		t.Fatalf("Lookup(claude): %v", err)
	}
	legacy := claude.LegacyUserFiles(Env{Home: "/home/u"})
	if len(legacy) != 1 || legacy[0] != "/home/u/.claude/commands/walden.md" {
		t.Fatalf("unexpected legacy files: %v", legacy)
	}
	for _, name := range []string{"codex", "copilot", "opencode"} {
		agent, err := Lookup(name)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", name, err)
		}
		if agent.LegacyUserFiles != nil {
			t.Fatalf("%s: expected no legacy files", name)
		}
	}
}
