package consolidate

// Inputs is one read of everything the consolidation state depends on.
type Inputs struct {
	Snapshot  Snapshot
	Record    *Record
	RecordErr error
	View      View
	ViewErr   error
}

// LoadInputs reads the portfolio snapshot, the record and the view without
// writing anything. Record and view failures are kept for State; only an
// unreadable specs directory is an error.
func LoadInputs(root string) (Inputs, error) {
	snapshot, err := LoadSnapshot(root)
	if err != nil {
		return Inputs{}, err
	}
	inputs := Inputs{Snapshot: snapshot}
	inputs.Record, inputs.RecordErr = LoadRecord(root)
	inputs.View, inputs.ViewErr = LoadView(root)
	return inputs, nil
}

// State derives the consolidation state; an unreadable view is an unknown state.
func (in Inputs) State() State {
	if in.ViewErr != nil && in.RecordErr == nil {
		return State{Tracking: TrackingUnknown, Threshold: ThresholdNone, View: ViewStatus{State: ViewDraft},
			Problem: in.ViewErr.Error(), Remedy: "fix the frontmatter of " + ViewPath + " or restore it from version control"}
	}
	return Derive(in.Snapshot, in.Record, in.RecordErr, in.View.Status())
}

// Report builds the read-only report, including view mismatches once
// tracking has started.
func (in Inputs) Report(root string) Report {
	state := in.State()
	report := BuildReport(root, in.Snapshot, in.Record, state)
	if state.Tracking == TrackingActive && in.View.Exists {
		report.Findings = append(report.Findings, CheckView(in.View, in.Snapshot, in.Record)...)
		report.Findings = append(report.Findings, CheckCoherence(ParseCoherenceReview(in.View.Document.Body), state, report.Comparisons)...)
	}
	return report
}
