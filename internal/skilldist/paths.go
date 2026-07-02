package skilldist

import (
	"errors"
	"os"
)

// Env carries the environment values that influence user-scope target paths.
// Resolution is pure given an Env value, so tests never touch the process
// environment.
type Env struct {
	Home          string
	CodexHome     string
	CopilotHome   string
	OpencodeHome  string
	XDGConfigHome string
}

// EnvFromOS captures the process environment for target resolution.
func EnvFromOS() Env {
	home, _ := os.UserHomeDir()
	return Env{
		Home:          home,
		CodexHome:     os.Getenv("CODEX_HOME"),
		CopilotHome:   os.Getenv("COPILOT_HOME"),
		OpencodeHome:  os.Getenv("OPENCODE_HOME"),
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
	}
}

func (e Env) home() (string, error) {
	if e.Home == "" {
		return "", errors.New("home directory is not resolvable")
	}
	return e.Home, nil
}
