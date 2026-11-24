// Package trace joins the statements of a feature with the proof steps that
// cover them and the evidence state of their leaves. It reads no file and
// runs no command: its inputs are what the gates already read.
package trace

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/andrearaponi/walden/internal/spec"
)

// CommandLimit is the number of characters of a command the text output and
// the Markdown table show.
const CommandLimit = 120

// The kinds of a listed statement.
const (
	KindCriterion  = "criterion"
	KindNFR        = "nfr"
	KindConstraint = "constraint"
)

// The kinds of a hole, in the order a report lists them: the criteria that
// lack a proof come before the proofs that are not standing.
const (
	HoleUnnamed     = "unnamed"
	HoleUncovered   = "uncovered"
	HoleNotStanding = "not-standing"
)

// Report is the trace of one feature.
type Report struct {
	Feature    string      `json:"feature"`
	Statements []Statement `json:"statements"`
	Leaves     []Leaf      `json:"leaves"`
	Holes      []Hole      `json:"holes"`
}

// Statement is one listed statement with the leaves that name it and the
// proof steps that cover it.
type Statement struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	NamedBy []string `json:"named_by"`
	Proofs  []Proof  `json:"proofs"`
}

// Proof is one proof step that covers a statement, with the evidence state
// of the leaf it belongs to.
type Proof struct {
	TaskID   string   `json:"task_id"`
	Step     int      `json:"step"`
	Command  []string `json:"command"`
	Delivery bool     `json:"delivery,omitempty"`
	State    string   `json:"state"`
}

// Leaf is one leaf of the plan, with its design references as the plan
// writes them.
type Leaf struct {
	ID         string   `json:"id"`
	DesignRefs []string `json:"design_refs"`
	Delivery   bool     `json:"delivery,omitempty"`
	State      string   `json:"state"`
}

// Hole is one gap of a trace: a criterion that no leaf names, a criterion
// that no proof step covers, or a proof step on a leaf that is not standing.
type Hole struct {
	Kind      string   `json:"kind"`
	Statement string   `json:"statement"`
	NamedBy   []string `json:"named_by,omitempty"`
	TaskID    string   `json:"task_id,omitempty"`
	Step      int      `json:"step,omitempty"`
	State     string   `json:"state,omitempty"`
}

// Inputs are the facts a trace joins.
type Inputs struct {
	Feature string
	// The identifiers the requirements define, each list in document order.
	Criteria, NFRs, Constraints []string
	// The leaves of the plan, in plan order.
	Leaves []*spec.Task
	// The evidence state of each leaf, by leaf identifier.
	States map[string]string
	// The reading walden validate judges: the criteria that no Requirements
	// line names and the criteria that no proof step covers.
	Unnamed, Uncovered []string
}

// The states of a leaf whose proof is standing. Every other state is a hole
// for the statements its steps cover.
const (
	stateVerified  = "verified"
	stateDelivered = "delivered"
)

// Build joins the inputs into a report. It lists every criterion in document
// order, then the non-functional requirements and the constraints that a
// leaf names or a proof step covers. An identifier that is no listed
// statement, a requirement heading or one the requirements do not define,
// adds no entry.
func Build(in Inputs) Report {
	report := Report{Feature: in.Feature, Statements: []Statement{}, Leaves: []Leaf{}, Holes: []Hole{}}

	// What the parser read: the leaves that name each identifier and the
	// steps that cover it, in plan order.
	namedBy := map[string][]string{}
	proofs := map[string][]Proof{}
	for _, leaf := range in.Leaves {
		state := in.States[leaf.ID]
		report.Leaves = append(report.Leaves, Leaf{
			ID:         leaf.ID,
			DesignRefs: append([]string{}, leaf.DesignRefs...),
			Delivery:   leaf.Proof.Delivery,
			State:      state,
		})
		named := map[string]bool{}
		for _, id := range leaf.Requirements {
			if !named[id] {
				named[id] = true
				namedBy[id] = append(namedBy[id], leaf.ID)
			}
		}
		for index, step := range leaf.Proof.Steps {
			covered := map[string]bool{}
			for _, id := range step.Covers {
				if covered[id] {
					continue
				}
				covered[id] = true
				proofs[id] = append(proofs[id], Proof{
					TaskID:   leaf.ID,
					Step:     index + 1,
					Command:  append([]string{}, step.Argv...),
					Delivery: leaf.Proof.Delivery,
					State:    state,
				})
			}
		}
	}

	// The holes are kept by kind and listed in that order.
	var withoutLeaf, withoutProof, notStanding []Hole
	unnamed, uncovered := set(in.Unnamed), set(in.Uncovered)
	// coverageHole reports a criterion the validator reads without a leaf or
	// without a proof: at most one of the two. The leaves it carries are the
	// ones the parser read, and the two readings can differ.
	coverageHole := func(id string) {
		switch {
		case unnamed[id]:
			withoutLeaf = append(withoutLeaf, Hole{Kind: HoleUnnamed, Statement: id, NamedBy: namedBy[id]})
		case uncovered[id]:
			withoutProof = append(withoutProof, Hole{Kind: HoleUncovered, Statement: id, NamedBy: namedBy[id]})
		}
	}

	listed := map[string]bool{}
	list := func(id, kind string) {
		statement := Statement{ID: id, Kind: kind, NamedBy: []string{}, Proofs: []Proof{}}
		statement.NamedBy = append(statement.NamedBy, namedBy[id]...)
		statement.Proofs = append(statement.Proofs, proofs[id]...)
		report.Statements = append(report.Statements, statement)
		listed[id] = true

		if kind == KindCriterion {
			coverageHole(id)
		}
		for _, proof := range statement.Proofs {
			if proof.State != stateVerified && proof.State != stateDelivered {
				notStanding = append(notStanding, Hole{Kind: HoleNotStanding, Statement: id, TaskID: proof.TaskID, Step: proof.Step, State: proof.State})
			}
		}
	}
	for _, id := range in.Criteria {
		list(id, KindCriterion)
	}
	for _, group := range []struct {
		ids  []string
		kind string
	}{{in.NFRs, KindNFR}, {in.Constraints, KindConstraint}} {
		for _, id := range group.ids {
			if len(namedBy[id]) > 0 || len(proofs[id]) > 0 {
				list(id, group.kind)
			}
		}
	}
	// A criterion the requirements mention and do not define is no listed
	// statement, and the validator counts it: its hole is reported all the
	// same, last in its kind.
	for _, ids := range [][]string{in.Unnamed, in.Uncovered} {
		for _, id := range ids {
			if !listed[id] {
				listed[id] = true
				coverageHole(id)
			}
		}
	}

	report.Holes = append(report.Holes, withoutLeaf...)
	report.Holes = append(report.Holes, withoutProof...)
	report.Holes = append(report.Holes, notStanding...)
	return report
}

func set(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

// HoleBreakdown counts the holes of a report by kind, in the order they are
// listed: "0 unnamed, 1 uncovered, 3 not-standing".
func (r Report) HoleBreakdown() string {
	counts := map[string]int{}
	for _, hole := range r.Holes {
		counts[hole.Kind]++
	}
	return fmt.Sprintf("%d %s, %d %s, %d %s",
		counts[HoleUnnamed], HoleUnnamed, counts[HoleUncovered], HoleUncovered, counts[HoleNotStanding], HoleNotStanding)
}

// Summary states the statements and the holes of a report, the holes by
// kind, so that a missing proof is told from a proof that is not standing.
func (r Report) Summary() string {
	summary := fmt.Sprintf("trace for %s: %d statement(s), %d hole(s)", r.Feature, len(r.Statements), len(r.Holes))
	if len(r.Holes) > 0 {
		summary += ": " + r.HoleBreakdown()
	}
	return summary
}

// Shown renders a command for a reader. The arguments are separated by one
// space; an argument that holds a space, a double quote or a character that
// is not printable is written as a Go string literal, so that the boundaries
// of the arguments stay visible and a line break does not break the line. A
// command of more than CommandLimit characters is cut to that many, the last
// one being an ellipsis.
func Shown(argv []string) string {
	parts := make([]string, len(argv))
	for index, argument := range argv {
		parts[index] = argument
		if needsQuoting(argument) {
			parts[index] = strconv.Quote(argument)
		}
	}
	text := []rune(strings.Join(parts, " "))
	if len(text) > CommandLimit {
		text = append(text[:CommandLimit-1:CommandLimit-1], '…')
	}
	return string(text)
}

func needsQuoting(argument string) bool {
	for _, r := range argument {
		if r == ' ' || r == '"' || !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}
