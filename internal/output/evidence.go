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
