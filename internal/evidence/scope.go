package evidence

import (
	"sort"
	"strings"
)

// Scope declares the assessed selection, never inferred business lifecycle.
type Scope struct {
	Kind      string   `json:"kind"`
	Features  []string `json:"features"`
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

func (s Scope) NamedFeature() string {
	if s.Kind == "feature" && len(s.Features) == 1 {
		return s.Features[0]
	}
	return ""
}

func (s Scope) Description() string { return s.Kind + ": " + strings.Join(s.Features, ", ") }

func (entry TaskEvidence) GapSummary() string {
	messages := make([]string, 0, len(entry.Gaps))
	for _, gap := range entry.Gaps {
		messages = append(messages, gap.Message)
	}
	return strings.Join(messages, "; ")
}
