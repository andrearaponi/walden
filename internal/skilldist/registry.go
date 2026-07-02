// Package skilldist places, removes, and inspects the embedded Walden skill
// across supported coding agents. All operations are offline and
// deterministic: targets come from a static registry resolved against an
// explicit environment snapshot.
package skilldist

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Scope selects where a skill installation lives.
type Scope string

const (
	// ScopeUser installs under the user's home configuration.
	ScopeUser Scope = "user"
	// ScopeProject installs under the current working directory.
	ScopeProject Scope = "project"
)

// WriteKind selects the placement strategy for an agent.
type WriteKind string

const (
	// KindFile writes SKILL.md as a standalone file.
	KindFile WriteKind = "file"
	// KindBlock maintains a marker-delimited block inside a shared file.
	KindBlock WriteKind = "block"
)

// ErrUnknownAgent reports an agent name outside the registry.
var ErrUnknownAgent = errors.New("unknown agent")

// Agent describes one supported coding agent.
type Agent struct {
	Name string
	Kind WriteKind
	// UserTarget resolves the user-scope target path from the environment.
	UserTarget func(Env) (string, error)
	// ProjectTarget resolves the project-scope target path under workDir.
	// The second return is false when the agent supports only user scope.
	ProjectTarget func(workDir string) (string, bool)
	// LegacyUserFiles lists obsolete artifacts removed on user-scope
	// install and uninstall.
	LegacyUserFiles func(Env) []string
}

var agents = []Agent{
	{
		Name: "claude",
		Kind: KindFile,
		UserTarget: func(env Env) (string, error) {
			home, err := env.home()
			if err != nil {
				return "", err
			}
			return filepath.Join(home, ".claude", "skills", "walden", "SKILL.md"), nil
		},
		ProjectTarget: func(workDir string) (string, bool) {
			return filepath.Join(workDir, ".claude", "skills", "walden", "SKILL.md"), true
		},
		LegacyUserFiles: func(env Env) []string {
			home, err := env.home()
			if err != nil {
				return nil
			}
			return []string{filepath.Join(home, ".claude", "commands", "walden.md")}
		},
	},
	{
		Name: "codex",
		Kind: KindBlock,
		UserTarget: func(env Env) (string, error) {
			if env.CodexHome != "" {
				return filepath.Join(env.CodexHome, "AGENTS.md"), nil
			}
			home, err := env.home()
			if err != nil {
				return "", err
			}
			return filepath.Join(home, ".codex", "AGENTS.md"), nil
		},
		ProjectTarget: func(workDir string) (string, bool) {
			return filepath.Join(workDir, "AGENTS.md"), true
		},
	},
	{
		Name: "copilot",
		Kind: KindFile,
		UserTarget: func(env Env) (string, error) {
			if env.CopilotHome != "" {
				return filepath.Join(env.CopilotHome, "skills", "walden", "SKILL.md"), nil
			}
			home, err := env.home()
			if err != nil {
				return "", err
			}
			return filepath.Join(home, ".copilot", "skills", "walden", "SKILL.md"), nil
		},
		ProjectTarget: func(string) (string, bool) { return "", false },
	},
	{
		Name: "opencode",
		Kind: KindFile,
		UserTarget: func(env Env) (string, error) {
			base := env.OpencodeHome
			if base == "" {
				if env.XDGConfigHome != "" {
					base = filepath.Join(env.XDGConfigHome, "opencode")
				} else {
					home, err := env.home()
					if err != nil {
						return "", err
					}
					base = filepath.Join(home, ".config", "opencode")
				}
			}
			return filepath.Join(base, "skills", "walden", "SKILL.md"), nil
		},
		ProjectTarget: func(string) (string, bool) { return "", false },
	},
}

// Agents returns the supported agents in deterministic registry order.
func Agents() []Agent {
	return append([]Agent(nil), agents...)
}

// AgentNames returns the supported agent names in registry order.
func AgentNames() []string {
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	return names
}

// Lookup returns the agent registered under name.
func Lookup(name string) (Agent, error) {
	for _, agent := range agents {
		if agent.Name == name {
			return agent, nil
		}
	}
	return Agent{}, fmt.Errorf("%w: %q (supported: %s)", ErrUnknownAgent, name, strings.Join(AgentNames(), ", "))
}
