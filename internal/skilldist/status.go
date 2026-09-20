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
	StateUnreadable   = "unreadable"
)

// SkillStatus describes one agent×scope installation slot.
type SkillStatus struct {
	Agent string
	Scope Scope
	Path  string
	// Installed preserves the legacy slot flag, including read failures.
	// State determines readability; this flag does not establish ownership.
	Installed bool
	State     string
	Version   string
}

// Status scans every agent and scope combination and classifies each
// installation against the embedded skill. The second return carries
// warnings (dual-scope divergence, unresolvable targets). Status never
// mutates anything: it is a report, not a gate.
func Status(opts Options) ([]SkillStatus, []string) {
	return statusWithReader(opts, os.ReadFile)
}

func statusWithReader(opts Options, readFile func(string) ([]byte, error)) ([]SkillStatus, []string) {
	embedded := normalizeBody(skill.Content())
	statuses := make([]SkillStatus, 0, len(agents)*2)
	warnings := []string{}

	for _, agent := range agents {
		bodies := map[Scope][]byte{}

		for _, scope := range []Scope{ScopeUser, ScopeProject} {
			status, body, comparable, warning := statusFor(agent, scope, opts, embedded, readFile)
			if warning != "" {
				warnings = append(warnings, warning)
			}
			if status == nil {
				continue
			}
			statuses = append(statuses, *status)
			if comparable {
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

// statusFor returns a comparable body only after a successful read and block
// extraction. A readable empty body is comparable; an unavailable body is not.
// Unsupported scopes retain their existing nil-status behavior.
func statusFor(agent Agent, scope Scope, opts Options, embedded []byte, readFile func(string) ([]byte, error)) (*SkillStatus, []byte, bool, string) {
	target, err := resolveTarget(agent, scope, opts)
	if err != nil {
		if scope == ScopeProject {
			return nil, nil, false, ""
		}
		return nil, nil, false, fmt.Sprintf("agent %s: %v", agent.Name, err)
	}

	status := SkillStatus{Agent: agent.Name, Scope: scope, Path: target, State: StateNotInstalled}

	raw, found, err := readInstalled(agent, target, readFile)
	if err != nil {
		status.Installed = true
		if errors.Is(err, ErrCorruptBlock) {
			// Successfully read malformed content remains a structural drift,
			// but no body was extracted for a user/project comparison.
			status.State = StateDrifted
			return &status, nil, false, fmt.Sprintf("agent %s (%s): %v", agent.Name, scope, err)
		}
		status.State = StateUnreadable
		return &status, nil, false, fmt.Sprintf("agent %s (%s), path %s: %v; content comparison not performed", agent.Name, scope, target, err)
	}
	if !found {
		return &status, nil, false, ""
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
	return &status, normalized, true, ""
}

// readInstalled returns the comparable installed content for the slot: the
// whole file for file-kind agents, the block interior for block-kind ones.
func readInstalled(agent Agent, target string, readFile func(string) ([]byte, error)) (raw []byte, found bool, err error) {
	data, err := readFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Normalize a comparison view before block and stamp parsing. Writers and
	// their raw-byte parser helpers must never receive this normalized view.
	data = comparisonLines(data)
	if agent.Kind == KindBlock {
		return blockInterior(data, target)
	}
	return data, true, nil
}

// comparisonLines treats CRLF as LF, without altering the original buffer or
// accepting other whitespace/encoding changes (including lone CR or a BOM).
func comparisonLines(data []byte) []byte {
	if !bytes.Contains(data, []byte("\r\n")) {
		return data
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// normalizeBody uses the same line-ending view for embedded/installed content,
// retaining the existing tolerance for trailing newlines only.
func normalizeBody(body []byte) []byte {
	return bytes.TrimRight(comparisonLines(body), "\n")
}
