package skill

import "testing"

// These assertions protect the distributed guide's stated contract. They do
// not establish that a model follows it; behavioral checks are separate.
func TestExecutionReportingSkillContract(t *testing.T) {
	text := string(canonicalGuide(t))
	for name, fragments := range map[string][]string{
		"behavioral chronology": {
			"TDD describes development order, not a passing proof",
			"runnable behavioral assertion fail before implementing",
			"minimal compilable scaffolding",
			"Compile/import failures and named PASS output do not establish that sequence",
		},
		"no retrospective reconstruction": {
			"tests added afterward or a test-driven repair",
			"Do not break and restore existing code to manufacture retrospective TDD",
			"label mutation testing separately",
		},
		"final state before handoff": {
			"finish code, README and other deliverables before the final scoped verification",
			"inspect the release verdict and report any deferred gaps",
		},
		"explain observed changes": {
			"Committing unchanged bytes does not by itself stale code evidence",
			"reported differences rather than guessing the cause",
		},
	} {
		t.Run(name, func(t *testing.T) {
			requireAuthoringText(t, text, fragments...)
		})
	}
}
