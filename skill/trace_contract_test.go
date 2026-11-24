package skill

import "testing"

// TestGuideListsTrace keeps the trace reachable from the guide: an agent
// following the command table would otherwise never run it, nor put its table
// in a pull request. The condition stays because the guide still supports
// older CLIs, where the command is unknown, and the row says who judges: a
// trace without holes is not a release verdict.
func TestGuideListsTrace(t *testing.T) {
	requireAuthoringText(t, string(canonicalGuide(t)),
		"| `walden trace <feature> [--markdown]` |",
		"On CLI v0.16.0 or newer",
		"list each acceptance criterion with the proof steps that cover it and their evidence state",
		"a criterion that no task names, a criterion that no proof step covers, a proof that is not standing",
		"Read-only; `release check` still judges",
		"`--markdown` prints a table for a pull request: run it after `walden verify <feature>`",
	)
}
