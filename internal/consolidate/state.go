package consolidate

import "sort"

// ViewPath is the current-contract view, relative to the repository root.
const ViewPath = ".walden/contracts.md"

// Tracking states.
const (
	TrackingNotStarted = "not-started"
	TrackingActive     = "tracking"
	TrackingUnknown    = "unknown"
)

// Thresholds.
const (
	ThresholdNone      = "none"
	ThresholdSuggested = "suggested"
	ThresholdDue       = "due"
)

// Pending change kinds.
const (
	ChangeNew     = "new"
	ChangeRevised = "revised"
	ChangeRemoved = "removed"
)

// View states.
const (
	ViewAbsent   = "absent"
	ViewDraft    = "draft"
	ViewInReview = "in-review"
	ViewApproved = "approved"
	ViewStale    = "stale"
)

// ViewStatus summarizes the view document's review state.
type ViewStatus struct {
	State               string
	ApprovedFingerprint string
}

// PendingChange is one feature whose contract changed since the last consolidation.
type PendingChange struct {
	Feature string
	Kind    string
}

// State is the derived consolidation state of a repository.
type State struct {
	Tracking       string
	Pending        []PendingChange
	Backlog        []string
	Unconsolidated int
	Threshold      string
	View           ViewStatus
	Problem        string
	Remedy         string
}

// Thresholds of pending contract changes.
const (
	SuggestAt = 2
	DueAt     = 3
)

// Derive computes the consolidation state from one snapshot, the record (or
// its load error) and the view status. It is pure: it reads and writes nothing.
func Derive(snapshot Snapshot, record *Record, recordErr error, view ViewStatus) State {
	state := State{Threshold: ThresholdNone, View: view}
	unknown := func(problem, remedy string) State {
		state.Tracking, state.Problem, state.Remedy = TrackingUnknown, problem, remedy
		return state
	}
	if recordErr != nil {
		return unknown(recordErr.Error(), "restore "+RecordPath+" from version control, or move it aside and run walden consolidate start again")
	}
	if record == nil {
		state.Tracking = TrackingNotStarted
		for _, feature := range snapshot.Features {
			if feature.Approved() {
				state.Unconsolidated++
			}
		}
		return state
	}
	if record.Checkpoint != nil {
		switch view.State {
		case ViewAbsent:
			return unknown("the current-contract view "+ViewPath+" is missing although "+RecordPath+" records a consolidation",
				"restore "+ViewPath+" from version control")
		case ViewApproved, ViewStale:
			if view.ApprovedFingerprint != record.Checkpoint.ViewFingerprint {
				return unknown(RecordPath+" does not match the approval of "+ViewPath,
					"run walden consolidate approve again on the approved view, or restore both files from version control")
			}
		}
	}

	state.Tracking = TrackingActive
	present := map[string]bool{}
	for _, feature := range snapshot.Features {
		present[feature.Name] = true
		if !feature.Approved() {
			continue
		}
		entry, recorded := record.Features[feature.Name]
		switch {
		case !recorded:
			state.Pending = append(state.Pending, PendingChange{Feature: feature.Name, Kind: ChangeNew})
		case entry.RequirementsFingerprint != feature.ApprovedFingerprint:
			state.Pending = append(state.Pending, PendingChange{Feature: feature.Name, Kind: ChangeRevised})
		case entry.State == StateBacklog:
			state.Backlog = append(state.Backlog, feature.Name)
		}
	}
	for name := range record.Features {
		if !present[name] {
			state.Pending = append(state.Pending, PendingChange{Feature: name, Kind: ChangeRemoved})
		}
	}
	sort.Slice(state.Pending, func(i, j int) bool { return state.Pending[i].Feature < state.Pending[j].Feature })
	sort.Strings(state.Backlog)

	switch {
	case len(state.Pending) >= DueAt:
		state.Threshold = ThresholdDue
	case len(state.Pending) >= SuggestAt:
		state.Threshold = ThresholdSuggested
	}
	return state
}
