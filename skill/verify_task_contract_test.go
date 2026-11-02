package skill

import "testing"

// TestGuideListsVerifyTask keeps the repair path reachable from the guide: an
// agent following the command table would otherwise re-prove the whole feature.
// The condition stays because the guide still supports older CLIs, where the
// flag is unknown.
func TestGuideListsVerifyTask(t *testing.T) {
	requireAuthoringText(t, string(canonicalGuide(t)),
		"| `walden verify <feature> --task <id>` |",
		"On CLI v0.14.2 or newer",
		"re-prove one completed leaf task after a targeted repair",
		"`release check` still judges every task, so finish with `walden verify <feature>`",
	)
}
