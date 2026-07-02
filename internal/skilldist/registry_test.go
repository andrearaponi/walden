package skilldist

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistryOrderAndNames(t *testing.T) {
	want := []string{"claude", "codex", "copilot", "opencode"}
	got := AgentNames()
	if len(got) != len(want) {
		t.Fatalf("expected %d agents, got %d", len(want), len(got))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("agent %d: expected %s, got %s", i, name, got[i])
		}
	}
}

func TestRegistryLookupKnownAgents(t *testing.T) {
	kinds := map[string]WriteKind{
		"claude":   KindFile,
		"codex":    KindBlock,
		"copilot":  KindFile,
		"opencode": KindFile,
	}
	for name, kind := range kinds {
		agent, err := Lookup(name)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", name, err)
		}
		if agent.Kind != kind {
			t.Fatalf("Lookup(%s): expected kind %s, got %s", name, kind, agent.Kind)
		}
	}
}

func TestRegistryLookupUnknownAgent(t *testing.T) {
	_, err := Lookup("cursor")
	if !errors.Is(err, ErrUnknownAgent) {
		t.Fatalf("expected ErrUnknownAgent, got %v", err)
	}
	if !strings.Contains(err.Error(), "claude, codex, copilot, opencode") {
		t.Fatalf("error must list supported agents, got: %v", err)
	}
}

func TestRegistryProjectScopeSupport(t *testing.T) {
	cases := []struct {
		agent     string
		supported bool
		target    string
	}{
		{"claude", true, "/work/.claude/skills/walden/SKILL.md"},
		{"codex", true, "/work/AGENTS.md"},
		{"copilot", false, ""},
		{"opencode", false, ""},
	}
	for _, tc := range cases {
		agent, err := Lookup(tc.agent)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", tc.agent, err)
		}
		target, ok := agent.ProjectTarget("/work")
		if ok != tc.supported {
			t.Fatalf("%s: expected project support %t, got %t", tc.agent, tc.supported, ok)
		}
		if ok && target != tc.target {
			t.Fatalf("%s: expected project target %s, got %s", tc.agent, tc.target, target)
		}
	}
}
