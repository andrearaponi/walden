package evidence

import (
	"sort"
	"strings"
)

// Scope declares the assessed selection, never inferred business lifecycle.
type Scope struct {
	Kind      string   `json:"kind"`
	Features  []string `json:"features"`
	Tasks     []string `json:"tasks,omitempty"`
	Guarantee string   `json:"guarantee"`
}

func NewScope(named bool, features ...string) Scope {
	names := append([]string{}, features...)
	sort.Strings(names)
	kind := "portfolio"
	if named {
		kind = "feature"
	}
	return Scope{Kind: kind, Features: names, Guarantee: Guarantee}
}

// NewTaskScope declares a selection of one task of one feature. Only the kind
// narrows: features and the guarantee stay those of a plain run.
func NewTaskScope(feature, task string) Scope {
	scope := NewScope(true, feature)
	scope.Kind, scope.Tasks = "task", []string{task}
	return scope
}

func (s Scope) NamedFeature() string {
	if s.Kind == "feature" && len(s.Features) == 1 {
		return s.Features[0]
	}
	return ""
}

func (s Scope) Description() string {
	features := strings.Join(s.Features, ", ")
	if s.Kind == "task" {
		return "task: " + strings.Join(s.Tasks, ", ") + " of " + features
	}
	return s.Kind + ": " + features
}

func (entry TaskEvidence) GapSummary() string {
	messages := make([]string, 0, len(entry.Gaps))
	for _, gap := range entry.Gaps {
		messages = append(messages, gap.Message)
	}
	return strings.Join(messages, "; ")
}
