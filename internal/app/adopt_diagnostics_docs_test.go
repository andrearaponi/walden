package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrearaponi/walden/internal/adopt"
	"github.com/andrearaponi/walden/internal/workflow"
)

func adoptDocumentSection(t *testing.T, content, start, end string) string {
	t.Helper()
	_, section, found := strings.Cut(content, start)
	if !found {
		t.Fatalf("missing documentation section %q", start)
	}
	section, _, _ = strings.Cut(section, end)
	return section
}

func adoptDocumentValue(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestAdoptDiagnosticsDocumentation(t *testing.T) {
	root := authoringSourceRoot(t)
	documents := map[string]string{}
	for _, name := range []string{"docs/adoption.md", "docs/reference/cli.md", "docs/reference/json.md", "skill/walden/SKILL.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		documents[name] = string(data)
	}
	reference := adoptDocumentSection(t, documents["docs/reference/json.md"], "## Adoption (`adopt --json`)", "## Error paths")
	for _, field := range []string{
		"result.adoption.workload", "assessed_tasks", "assessed_steps", "unassessed_features",
		"features[].workload", "features[].evidence", "features[].elapsed_ms", "features[].evidence_persisted", "evidence[].elapsed_ms",
	} {
		if !strings.Contains(reference, field) {
			t.Errorf("JSON reference omits %s", field)
		}
	}
	guide := adoptDocumentSection(t, documents["skill/walden/SKILL.md"], "### Adoption and retirement", "### Release judgment")
	cli := adoptDocumentSection(t, documents["docs/reference/cli.md"], "### `walden adopt", "## Certification")
	for label, check := range map[string]struct {
		text  string
		terms []string
	}{
		"adoption guide": {documents["docs/adoption.md"], []string{"declared steps", "unavailable", "not an ETA", "evidence_persisted", "accepted execution", "raw output", "no automatic"}},
		"CLI reference":  {cli, []string{"no proof or environment probe", "declared steps", "elapsed_ms", "evidence_persisted", "one JSON envelope"}},
		"skill guidance": {guide, []string{"non-executing plan", "declared steps", "elapsed_ms", "evidence_persisted", "older compatible CLIs", "No upgrade or inspection automatically replays"}},
	} {
		for _, term := range check.terms {
			if !strings.Contains(check.text, term) {
				t.Errorf("%s omits %q", label, term)
			}
		}
	}
	if !strings.Contains(documents["skill/walden/SKILL.md"], "Requires Walden CLI **v0.10.4 or a newer compatible release**") {
		t.Error("diagnostics unexpectedly raised the guide prerequisite")
	}
	for _, forbidden := range []string{"--affected", "--only-kind", "--amend-proof", "--save-logs"} {
		if strings.Contains(cli, forbidden) || strings.Contains(guide, forbidden) {
			t.Errorf("diagnostic guidance invented %s", forbidden)
		}
	}

	parts := strings.Split(reference, "```json\n")
	if len(parts) != 5 {
		t.Fatalf("expected four reviewed JSON examples, got %d", len(parts)-1)
	}
	duration := 125 * time.Millisecond
	persisted := false
	result := adoptApplyResult(adopt.ApplyReport{Features: []adopt.FeatureAdoption{{
		Name: "sample", Class: adopt.ClassReprove, Error: "persist refreshed evidence: write failed",
		EvidencePersisted: &persisted, Workload: adopt.Workload{Available: true, Tasks: 1, Steps: 1},
		Outcomes: []workflow.VerifyOutcome{{TaskID: "1", Passed: true, State: "verified", Elapsed: &duration}},
	}}})
	projected := adoptDocumentValue(t, result.Adoption.Features[0]).(map[string]any)
	unpersisted := map[string]any{}
	for _, key := range []string{"feature", "class", "reason", "evidence_persisted", "evidence"} {
		unpersisted[key] = projected[key]
	}
	expected := []any{
		adoptionWorkloadSummary(adopt.WorkloadSummary{AssessedTasks: 2, AssessedSteps: 5, UnassessedFeatures: []string{"blocked-feature"}}),
		adoptionWorkload(adopt.Workload{Available: true}),
		adoptionWorkload(adopt.Workload{Reason: "tasks assessment unavailable: malformed proof"}),
		unpersisted,
	}
	for i, part := range parts[1:] {
		text, _, found := strings.Cut(part, "```")
		if !found {
			t.Fatalf("unterminated example %d", i+1)
		}
		var actual any
		if err := json.Unmarshal([]byte(text), &actual); err != nil {
			t.Fatalf("example %d: %v", i+1, err)
		}
		if want := adoptDocumentValue(t, expected[i]); !reflect.DeepEqual(actual, want) {
			t.Errorf("example %d differs from production projection:\ngot %#v\nwant %#v", i+1, actual, want)
		}
	}
}
