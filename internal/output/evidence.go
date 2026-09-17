package output

import (
	"fmt"
	"io"

	"github.com/andrearaponi/walden/internal/evidence"
)

func printEvidence(w io.Writer, entries []EvidenceStatus, indent string) {
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
