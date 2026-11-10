package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/andrearaponi/walden/internal/evidence"
)

func printEvidence(w io.Writer, entries []EvidenceStatus, indent string) {
	printEvidenceWithRunDetails(w, entries, indent, false)
}

func printEvidenceWithRunDetails(w io.Writer, entries []EvidenceStatus, indent string, runDetails bool) {
	for _, entry := range entries {
		_, _ = fmt.Fprintf(w, "%s- %s: %s", indent, entry.TaskID, entry.State)
		if entry.Failure != "" {
			_, _ = fmt.Fprintf(w, " (%s)", entry.Failure)
		}
		if entry.ProfileLegacy {
			_, _ = fmt.Fprintf(w, " (legacy record: no profile)")
		}
		if entry.Binding != nil && entry.Execution != nil {
			_, _ = fmt.Fprintf(w, " [binding=%s code=%s execution=%s]", entry.Binding.State, entry.CodeFreshness, entry.Execution.State)
		}
		if runDetails {
			if entry.Passed != nil {
				_, _ = fmt.Fprintf(w, " passed=%t", *entry.Passed)
			}
			if entry.ElapsedMS != nil {
				_, _ = fmt.Fprintf(w, " elapsed=%d ms", *entry.ElapsedMS)
			}
			if entry.Execution != nil && entry.Execution.Facts != nil {
				facts := entry.Execution.Facts
				_, _ = fmt.Fprintf(w, " assertion=%s integrity=%s", facts.AssertionResult, facts.Integrity)
				if facts.CauseTask != "" {
					_, _ = fmt.Fprintf(w, " cause=%s", facts.CauseTask)
				}
				if len(facts.ChangedPaths) > 0 {
					_, _ = fmt.Fprintf(w, " changed=%s", strings.Join(facts.ChangedPaths, ", "))
				}
			}
		}
		_, _ = fmt.Fprintln(w)
		for _, gap := range entry.Gaps {
			_, _ = fmt.Fprintf(w, "%s  %s: %s\n", indent, gap.Kind, gap.Message)
		}
		for _, drift := range entry.ProfileDrift {
			_, _ = fmt.Fprintf(w, "%s  profile drift: %s: recorded %q → current %q\n", indent, drift.Key, drift.Recorded, drift.Current)
		}
		printExplanation(w, entry.Explanation, indent)
		if entry.RecordedDurationMS != nil {
			_, _ = fmt.Fprintf(w, "%s  recorded duration: %d ms\n", indent, *entry.RecordedDurationMS)
		}
	}
}

// displayLimit bounds every list an explanation prints in text mode: the
// first items in order, then the number omitted. JSON carries whole lists.
const displayLimit = 10

func boundedList(items []string) string {
	if len(items) <= displayLimit {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(items[:displayLimit], ", "), len(items)-displayLimit)
}

// printExplanation adds, under an entry, what changed since its record was
// written, or the one sentence that says why that cannot be established.
func printExplanation(w io.Writer, explanation *evidence.Explanation, indent string) {
	if explanation == nil {
		return
	}
	if code := explanation.Code; code != nil {
		if code.Unavailable != "" {
			_, _ = fmt.Fprintf(w, "%s  changed paths unknown: %s\n", indent, code.Detail)
		} else {
			paths := make([]string, 0, len(code.Paths))
			for _, change := range code.Paths {
				paths = append(paths, change.Kind+" "+change.Path)
			}
			_, _ = fmt.Fprintf(w, "%s  since commit %s: %s\n", indent, code.RecordedCommit, boundedList(paths))
		}
	}
	if spec := explanation.Spec; spec != nil {
		approvals := make([]string, 0, len(spec.Approvals))
		for _, approval := range spec.Approvals {
			if approval == evidence.ApprovalTask {
				approval = "task definition"
			}
			approvals = append(approvals, approval)
		}
		_, _ = fmt.Fprintf(w, "%s  changed approvals: %s\n", indent, strings.Join(approvals, ", "))
		switch {
		case spec.Unavailable != "":
			_, _ = fmt.Fprintf(w, "%s  changed statements unknown: %s\n", indent, spec.Detail)
		case spec.StatementsUnchanged:
			_, _ = fmt.Fprintf(w, "%s  changed statements: none of the statements this task names\n", indent)
		case len(spec.Statements) > 0:
			statements := make([]string, 0, len(spec.Statements))
			for _, change := range spec.Statements {
				kind := change.Kind
				if kind == evidence.KindRemoved {
					kind = "no longer defined"
				}
				statements = append(statements, change.ID+" "+kind)
			}
			_, _ = fmt.Fprintf(w, "%s  changed statements: %s\n", indent, boundedList(statements))
		}
	}
}

// EvidenceView keeps the status, verify, adoption and release projections in
// one place. A stored pass is not a statement about current assurance.
func EvidenceView(entry evidence.TaskEvidence) EvidenceStatus {
	view := EvidenceStatus{
		TaskID: entry.TaskID, State: entry.State,
		RecordedIdentity: entry.RecordedIdentity, CurrentIdentity: entry.CurrentIdentity,
		Profile: entry.RecordedProfile, ProfileLegacy: entry.ProfileLegacy,
		CodeFreshness: entry.CodeFreshness, Gaps: entry.Gaps,
		Explanation: entry.Explanation, RecordedDurationMS: entry.RecordedDurationMS,
	}
	if entry.Binding.State != "" {
		binding := entry.Binding
		view.Binding = &binding
	}
	if entry.Execution.State != "" {
		execution := entry.Execution
		view.Execution = &execution
	}
	if entry.RecordedResult != "" {
		passed := entry.RecordedResult == evidence.ResultPassed
		view.Passed = &passed
	}
	for _, drift := range entry.ProfileDrift {
		view.ProfileDrift = append(view.ProfileDrift, ProfileDriftEntry{Key: drift.Key, Recorded: drift.Recorded, Current: drift.Current})
	}
	return view
}

// printCurrentTree says what the current code identity is made of, so that
// an untracked copy or a forgotten edit is visible as an input. It names
// paths and applies nothing to them.
func printCurrentTree(w io.Writer, current *evidence.CurrentTree) {
	if current == nil {
		return
	}
	base := "commit " + current.Commit
	if current.Commit == "" {
		base = "no commit yet"
	}
	switch {
	case len(current.Paths) == 0:
		_, _ = fmt.Fprintf(w, "Current identity: %s, no uncommitted path\n", base)
		return
	case current.Commit == "":
		_, _ = fmt.Fprintf(w, "Current identity: %s, %d uncommitted path(s)\n", base, len(current.Paths))
	default:
		_, _ = fmt.Fprintf(w, "Current identity: %s plus %d uncommitted path(s)\n", base, len(current.Paths))
	}
	for index, path := range current.Paths {
		if index == displayLimit {
			_, _ = fmt.Fprintf(w, "- +%d more\n", len(current.Paths)-displayLimit)
			break
		}
		kind := path.Kind
		if path.Untracked {
			kind += " (untracked)"
		}
		_, _ = fmt.Fprintf(w, "- %s %s\n", kind, path.Path)
	}
}
