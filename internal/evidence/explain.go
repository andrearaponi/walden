package evidence

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// Kinds of a difference between two trees.
const (
	KindChanged = "changed"
	KindAdded   = "added"
	KindRemoved = "removed"
)

// Reasons an explanation is unavailable. Each names the fact that is
// missing: an explanation is exact or it is withheld.
const (
	ReasonNoCommit                = "no-commit"
	ReasonPathsNotKept            = "paths-not-kept"
	ReasonCommitMissing           = "commit-missing"
	ReasonIdentityMismatch        = "identity-mismatch"
	ReasonNoStatementFingerprints = "no-statement-fingerprints"
)

// Reasons the recorded pass of a delivery task is no attestation. With
// ReasonNoCommit they are checked in this order, and the first that holds is
// reported: the record keeps no tree facts, no commit could be read, the tree
// held uncommitted paths, the run did not read the proof as a delivery proof.
const (
	ReasonNoTree           = "no-tree"
	ReasonUncommittedPaths = "uncommitted-paths"
	ReasonUndeclaredRun    = "undeclared-run"
)

// AttestationExplanation says why the recorded pass of a completed delivery
// task is no attestation and what attests the task. Unavailable holds the
// reason code and Detail the sentence that goes with it.
type AttestationExplanation struct {
	Unavailable string `json:"unavailable"`
	Detail      string `json:"detail"`
}

// PathChange is one path that differs between two trees.
type PathChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// CodeExplanation says which paths changed in the tree since a record was
// written, or why that cannot be established. Unavailable holds a reason
// code and Detail the sentence that goes with it; Paths is empty then.
type CodeExplanation struct {
	RecordedCommit string       `json:"recorded_commit,omitempty"`
	Paths          []PathChange `json:"paths,omitempty"`
	Unavailable    string       `json:"unavailable,omitempty"`
	Detail         string       `json:"detail,omitempty"`
}

// Approvals an explanation can name as changed since a record.
const (
	ApprovalRequirements = "requirements"
	ApprovalDesign       = "design"
	ApprovalTask         = "task"
)

// StatementChange is one statement a task named that reads differently now:
// changed, or removed when the requirements no longer define it.
type StatementChange struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// SpecExplanation says which approvals changed since a record was written
// and, when the requirements did, what became of the statements its task
// named: those that changed or are gone, that none did, or why that cannot
// be established.
type SpecExplanation struct {
	Approvals           []string          `json:"approvals,omitempty"`
	Statements          []StatementChange `json:"statements,omitempty"`
	StatementsUnchanged bool              `json:"statements_unchanged,omitempty"`
	Unavailable         string            `json:"unavailable,omitempty"`
	Detail              string            `json:"detail,omitempty"`
}

// Explanation is the second reading of a record: it says what changed since
// the record was written. It sits beside the assessment Derive makes and
// never feeds it, so no explanation can move a state.
type Explanation struct {
	Code        *CodeExplanation        `json:"code,omitempty"`
	Spec        *SpecExplanation        `json:"spec,omitempty"`
	Attestation *AttestationExplanation `json:"attestation,omitempty"`
}

// CommitListing is what listing one recorded commit gave: the output of
// `git ls-tree -r`, or Missing when the commit cannot be read here.
type CommitListing struct {
	Listing string
	Missing bool
}

// ExplainInput carries what Explain needs beyond the record and its
// assessment. Every git result is passed in: Explain itself calls nothing.
type ExplainInput struct {
	// Feature names the feature in the sentence that says what records a
	// missing fact.
	Feature string
	// Current is the manifest of the current tree.
	Current Manifest
	// Listings holds the listing of each recorded commit, by commit.
	Listings map[string]CommitListing
	// Chain holds the current approval fingerprints.
	Chain ChainFingerprints
	// Statements holds the fingerprint of each statement the current
	// requirements define.
	Statements map[string]string
}

// Explain reads what changed since a record was written, following the
// independent dimensions of its assessment rather than the winning state.
// It returns nil when there is nothing to explain.
func Explain(record Record, entry TaskEvidence, input ExplainInput) *Explanation {
	// A delivered task is not explained. That the code and the documents
	// moved on since the delivery is what the state allows, not a difference
	// to account for.
	if entry.State == StateDelivered {
		return nil
	}
	explanation := &Explanation{
		Code:        explainCode(record, entry, input),
		Spec:        explainSpec(record, entry, input),
		Attestation: ExplainAttestation(record, entry, input.Feature),
	}
	if explanation.Code == nil && explanation.Spec == nil && explanation.Attestation == nil {
		return nil
	}
	return explanation
}

// explainSpec applies when an approval changed. It reads the same three
// comparisons Derive makes — the requirements fingerprint, the design
// fingerprint and the task binding — and adds none. A fingerprint absent on
// either side establishes no change.
func explainSpec(record Record, entry TaskEvidence, input ExplainInput) *SpecExplanation {
	changed := func(recorded, current string) bool {
		return recorded != "" && current != "" && recorded != current
	}
	requirementsChanged := changed(record.RequirementsFingerprint, input.Chain.Requirements)

	approvals := []string{}
	if requirementsChanged {
		approvals = append(approvals, ApprovalRequirements)
	}
	if changed(record.DesignFingerprint, input.Chain.Design) {
		approvals = append(approvals, ApprovalDesign)
	}
	if entry.Binding.State == "changed" {
		approvals = append(approvals, ApprovalTask)
	}
	if len(approvals) == 0 {
		return nil
	}

	explanation := &SpecExplanation{Approvals: approvals}
	if !requirementsChanged {
		return explanation
	}

	// Fingerprints this CLI cannot read are absent, never compared: across
	// two schemes every statement would read as changed. An entry read
	// through its attestation is explained from the statements its state
	// came from: a sealed record may keep none of its own.
	facts := record.Statements
	if entry.Binding.Source == "attestation" && record.Attestation != nil {
		facts = &record.Attestation.Statements
	}
	switch {
	case facts == nil:
		explanation.Unavailable = ReasonNoStatementFingerprints
		explanation.Detail = fmt.Sprintf("the record keeps no statement fingerprints, so the changed statements cannot be established; walden verify %s records them", input.Feature)
		return explanation
	case facts.Scheme != spec.StatementScheme:
		explanation.Unavailable = ReasonNoStatementFingerprints
		explanation.Detail = fmt.Sprintf("the record keeps statement fingerprints in the scheme %q, which this CLI does not read, so the changed statements cannot be established; walden verify %s records them", facts.Scheme, input.Feature)
		return explanation
	}

	ids := make([]string, 0, len(facts.Fingerprints))
	for id := range facts.Fingerprints {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return spec.StatementLess(ids[i], ids[j]) })
	for _, id := range ids {
		current, defined := input.Statements[id]
		switch {
		case !defined:
			explanation.Statements = append(explanation.Statements, StatementChange{ID: id, Kind: KindRemoved})
		case current != facts.Fingerprints[id]:
			explanation.Statements = append(explanation.Statements, StatementChange{ID: id, Kind: KindChanged})
		}
	}
	explanation.StatementsUnchanged = len(explanation.Statements) == 0
	return explanation
}

// explainCode applies when the code is stale. The reasons it cannot explain
// are checked in a fixed order, and the first that holds is reported: no
// commit, a path list that was not kept, a commit that is not here, facts
// that do not rebuild the recorded identity.
func explainCode(record Record, entry TaskEvidence, input ExplainInput) *CodeExplanation {
	if entry.CodeFreshness != "stale" {
		return nil
	}

	tree := record.Tree
	if tree == nil || tree.Commit == "" {
		return &CodeExplanation{
			Unavailable: ReasonNoCommit,
			Detail:      fmt.Sprintf("the record keeps no commit, so the changed paths cannot be established; walden verify %s records it", input.Feature),
		}
	}
	if tree.PathsOmitted > 0 {
		return &CodeExplanation{
			RecordedCommit: tree.Commit,
			Unavailable:    ReasonPathsNotKept,
			Detail: fmt.Sprintf("the record kept the number of differing paths (%d), not the paths, so the changed paths cannot be established; walden verify %s records them when at most %d differ",
				tree.PathsOmitted, input.Feature, RecordingLimit),
		}
	}

	listing, listed := input.Listings[tree.Commit]
	if !listed || listing.Missing {
		return &CodeExplanation{
			RecordedCommit: tree.Commit,
			Unavailable:    ReasonCommitMissing,
			Detail:         fmt.Sprintf("recorded commit %s is not in this repository, so the changed paths cannot be established", tree.Commit),
		}
	}

	// The recorded identity is the check on the facts: a tree rebuilt from
	// them is trusted only when it has the digest the record was bound to.
	rebuilt := RebuildManifest(listing.Listing, tree.Paths)
	if rebuilt.Digest() != record.CodeIdentity {
		return &CodeExplanation{
			RecordedCommit: tree.Commit,
			Unavailable:    ReasonIdentityMismatch,
			Detail:         "the recorded commit and paths do not reproduce the recorded code identity, so the changed paths cannot be established",
		}
	}

	return &CodeExplanation{RecordedCommit: tree.Commit, Paths: DiffEntries(rebuilt, input.Current)}
}

// DiffEntries returns every path whose entry differs between two manifests,
// in path order, as changed, added or removed.
func DiffEntries(before, after Manifest) []PathChange {
	changes := []PathChange{}
	for path, entry := range before {
		other, exists := after[path]
		switch {
		case !exists:
			changes = append(changes, PathChange{Path: path, Kind: KindRemoved})
		case other != entry:
			changes = append(changes, PathChange{Path: path, Kind: KindChanged})
		}
	}
	for path := range after {
		if _, exists := before[path]; !exists {
			changes = append(changes, PathChange{Path: path, Kind: KindAdded})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

// ListCommit lists the tree of a recorded commit. The commit comes from a
// ledger, which is data read from a file: only an object id reaches git, and
// anything else is missing without asking.
func ListCommit(ctx context.Context, runner shell.Runner, root, commit string) CommitListing {
	if !objectID(commit) {
		return CommitListing{Missing: true}
	}
	response, err := Git(ctx, runner, root, "ls-tree", "-r", commit)
	if err != nil || response.ExitCode != 0 {
		return CommitListing{Missing: true}
	}
	return CommitListing{Listing: response.Stdout}
}

// TreePath is one path that differs from the commit a tree rests on.
// Untracked marks a path git does not track: a copy or a forgotten file that
// is an input of the identity all the same.
type TreePath struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Untracked bool   `json:"untracked,omitempty"`
}

// CurrentTree is what the current code identity is made of: the commit and
// the paths outside .walden/ that differ from it. Commit is empty in a
// repository without one.
type CurrentTree struct {
	Commit string     `json:"commit,omitempty"`
	Paths  []TreePath `json:"paths,omitempty"`
}

// RecordedDuration is how long a recorded execution took: the sum of its
// steps, in milliseconds. It is a measurement of that one execution. A record
// with no step, or with a step that carries no duration, has none: absence
// is reported as absence, never as zero.
func RecordedDuration(record Record) *int64 {
	if len(record.Steps) == 0 {
		return nil
	}
	total := int64(0)
	for _, step := range record.Steps {
		if step.DurationMS == nil {
			return nil
		}
		total += *step.DurationMS
	}
	return &total
}
