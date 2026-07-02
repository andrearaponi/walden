package skilldist

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/andrearaponi/walden/skill"
)

// Installation states reported by Status.
const (
	StateNotInstalled = "not-installed"
	StateInSync       = "in-sync"
	StateDrifted      = "drifted"
)

// SkillStatus describes one agent×scope installation slot.
type SkillStatus struct {
	Agent     string
	Scope     Scope
	Path      string
	Installed bool
	State     string
	Version   string
}

// Status scans every agent and scope combination and classifies each
// installation against the embedded skill. The second return carries
// warnings (dual-scope divergence, unresolvable targets). Status never
// mutates anything: it is a report, not a gate.
func Status(opts Options) ([]SkillStatus, []string) {
	embedded := normalizeBody(skill.Content())
	statuses := make([]SkillStatus, 0, len(agents)*2)
	warnings := []string{}

	for _, agent := range agents {
		bodies := map[Scope][]byte{}

		for _, scope := range []Scope{ScopeUser, ScopeProject} {
			status, body, warning := statusFor(agent, scope, opts, embedded)
			if warning != "" {
				warnings = append(warnings, warning)
			}
			if status == nil {
				continue
			}
			statuses = append(statuses, *status)
			if status.Installed {
				bodies[scope] = body
			}
		}

		userBody, hasUser := bodies[ScopeUser]
		projectBody, hasProject := bodies[ScopeProject]
		if hasUser && hasProject && !bytes.Equal(userBody, projectBody) {
			warnings = append(warnings, fmt.Sprintf("agent %s: user-scope and project-scope installations differ; the agent will load only one of them", agent.Name))
		}
	}

	return statuses, warnings
}

// statusFor classifies one agent×scope slot. It returns a nil status when
// the scope is unsupported for the agent.
func statusFor(agent Agent, scope Scope, opts Options, embedded []byte) (*SkillStatus, []byte, string) {
	target, err := resolveTarget(agent, scope, opts)
	if err != nil {
		if scope == ScopeProject {
			return nil, nil, ""
		}
		return nil, nil, fmt.Sprintf("agent %s: %v", agent.Name, err)
	}

	status := SkillStatus{Agent: agent.Name, Scope: scope, Path: target, State: StateNotInstalled}

	raw, found, err := readInstalled(agent, target)
	if err != nil {
		// A corrupt block is an installation we cannot equate with the
		// embedded skill: report it as drifted rather than erroring.
		status.Installed = true
		status.State = StateDrifted
		return &status, nil, fmt.Sprintf("agent %s: %v", agent.Name, err)
	}
	if !found {
		return &status, nil, ""
	}

	body, version := Strip(raw)
	normalized := normalizeBody(body)
	status.Installed = true
	status.Version = version
	if bytes.Equal(normalized, embedded) {
		status.State = StateInSync
	} else {
		status.State = StateDrifted
	}
	return &status, normalized, ""
}

// readInstalled returns the comparable installed content for the slot: the
// whole file for file-kind agents, the block interior for block-kind ones.
func readInstalled(agent Agent, target string) (raw []byte, found bool, err error) {
	data, err := os.ReadFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if agent.Kind == KindBlock {
		return blockInterior(data, target)
	}
	return data, true, nil
}

// normalizeBody strips trailing newlines so cosmetic differences (such as
// the extra newline legacy setup.sh blocks carry) do not read as drift.
func normalizeBody(body []byte) []byte {
	return bytes.TrimRight(body, "\n")
}
