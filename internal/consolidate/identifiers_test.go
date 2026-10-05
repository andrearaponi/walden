package consolidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const definitionsBody = "# Requirements Document\n\n" +
	"## Introduction\n\n" +
	"Retirement follows `.walden/RETIRED.md`; the gate stays `release-check#R1.AC1`.\n\n" +
	"### Impact On Existing Contracts\n\n" +
	"| Operation | Existing contract | Proposed effect |\n" +
	"| --- | --- | --- |\n" +
	"| Preserve | `billing#C1`, `billing#NFR2` | Invoices unchanged. |\n\n" +
	"## Requirements\n\n" +
	"### R1 Booking\n\n" +
	"#### Acceptance Criteria\n\n" +
	"1. `R1.AC1` WHEN a member books a free desk, the system SHALL confirm it while `desk-occupancy#R1.AC2` holds (bridged by `NFR1`).\n" +
	"   - Acceptance check: `R1.AC9`, `docs/example/fake.md` and `other#R9.AC9` are only examples here.\n" +
	"2. `R1.AC2` WHEN a holder cancels, the system SHALL release the desk.\n\n" +
	"### R2 Records\n\n" +
	"1. `R2.AC1` The system SHALL keep one record per booking.\n\n" +
	"## Non-Functional Requirements\n\n" +
	"- `NFR1` Privacy: no names in records (bridged by `R2.AC1`).\n\n" +
	"## Constraints And Dependencies\n\n" +
	"- `C1` Standard library only; run `walden consolidate`, keep `go.mod`, ignore `https://example.com/a.md`, `/abs/path.md`, `../outside.md`, `cmd/walden@v0.10.4` and `docs/decisions/`.\n" +
	"- `C2` Bookable hours follow decision K9 (`docs/decisions/K9-opening-hours.md`), cited again as `docs/decisions/K9-opening-hours.md`.\n\n" +
	"```markdown\n" +
	"1. `R9.AC1` WHEN fenced, the system SHALL not count `docs/fenced.md` or `x#R1.AC1`.\n" +
	"- `C9` A fenced constraint.\n" +
	"```\n"

func TestDefinedIdentifiers(t *testing.T) {
	got := DefinedIdentifiers(definitionsBody)
	want := Definitions{
		Requirements: []string{"R1", "R2"},
		Criteria:     []string{"R1.AC1", "R1.AC2", "R2.AC1"},
		NFRs:         []string{"NFR1"},
		Constraints:  []string{"C1", "C2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("definitions = %+v, want %+v", got, want)
	}
	if active := got.Active(); !reflect.DeepEqual(active, []string{"R1.AC1", "R1.AC2", "R2.AC1", "NFR1", "C1", "C2"}) {
		t.Fatalf("active identifiers = %v", active)
	}
	for id, defined := range map[string]bool{"R2": true, "R1.AC2": true, "NFR1": true, "C2": true, "R1.AC9": false, "R9.AC1": false, "C9": false, "NFR2": false} {
		if got.Contains(id) != defined {
			t.Errorf("Contains(%q) = %v, want %v", id, !defined, defined)
		}
	}
}

func TestCitations(t *testing.T) {
	got := ParseCitations(definitionsBody)
	wantFiles := []FileCitation{
		{Path: ".walden/RETIRED.md"},
		{Path: "docs/decisions/K9-opening-hours.md", CitedBy: "C2"},
	}
	wantReferences := []Reference{
		{Feature: "release-check", ID: "R1.AC1"},
		{Feature: "billing", ID: "C1"},
		{Feature: "billing", ID: "NFR2"},
		{Feature: "desk-occupancy", ID: "R1.AC2", CitedBy: "R1.AC1"},
	}
	if !reflect.DeepEqual(got.Files, wantFiles) {
		t.Errorf("cited files = %+v, want %+v", got.Files, wantFiles)
	}
	if !reflect.DeepEqual(got.References, wantReferences) {
		t.Errorf("qualified references = %+v, want %+v", got.References, wantReferences)
	}
}

func writeRequirements(t *testing.T, root, name, status, body string, fresh bool) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", name, "requirements.md")
	fingerprint := ""
	if status == "approved" {
		fingerprint = spec.Fingerprint(path, body)
	}
	if !fresh {
		body += "\nEdited after approval.\n"
	}
	document := spec.Document{Path: path, Body: body, Fields: map[string]string{
		"status": status, "approved_at": "", "last_modified": "2026-10-04T06:00:00Z", "approved_fingerprint": fingerprint,
	}}
	if status == "approved" {
		document.Fields["approved_at"] = "2026-10-04T06:00:00Z"
	}
	if err := spec.SaveDocument(document); err != nil {
		t.Fatal(err)
	}
}

func TestPortfolioSnapshot(t *testing.T) {
	t.Run("empty repository", func(t *testing.T) {
		snapshot, err := LoadSnapshot(t.TempDir())
		if err != nil || len(snapshot.Features) != 0 {
			t.Fatalf("snapshot = %+v, err = %v; want empty and no error", snapshot, err)
		}
	})

	root := t.TempDir()
	writeRequirements(t, root, "alpha", "approved", definitionsBody, true)
	writeRequirements(t, root, "beta", "approved", definitionsBody, false)
	writeRequirements(t, root, "gamma", "draft", definitionsBody, false)
	if err := os.MkdirAll(filepath.Join(root, ".walden", "specs", "delta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".walden", "specs", "notes.txt"), []byte("not a feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, feature := range snapshot.Features {
		names = append(names, feature.Name)
	}
	if !reflect.DeepEqual(names, []string{"alpha", "beta", "delta", "gamma"}) {
		t.Fatalf("features = %v", names)
	}

	alpha, ok := snapshot.Feature("alpha")
	if !ok || !alpha.HasRequirements || alpha.Status != "approved" || !alpha.Fresh {
		t.Fatalf("alpha = %+v, want approved, fresh requirements", alpha)
	}
	if alpha.ApprovedFingerprint != spec.Fingerprint("requirements.md", definitionsBody) {
		t.Errorf("alpha fingerprint = %q", alpha.ApprovedFingerprint)
	}
	if !reflect.DeepEqual(alpha.Definitions.Active(), []string{"R1.AC1", "R1.AC2", "R2.AC1", "NFR1", "C1", "C2"}) || len(alpha.Citations.References) != 4 {
		t.Errorf("alpha definitions or citations not parsed: %+v", alpha)
	}
	if beta, _ := snapshot.Feature("beta"); beta.Status != "approved" || beta.Fresh {
		t.Errorf("beta = %+v, want approved but not fresh", beta)
	}
	if gamma, _ := snapshot.Feature("gamma"); gamma.Status != "draft" || gamma.Fresh {
		t.Errorf("gamma = %+v, want draft", gamma)
	}
	if delta, _ := snapshot.Feature("delta"); delta.HasRequirements || delta.Fresh {
		t.Errorf("delta = %+v, want no requirements", delta)
	}
	if _, ok := snapshot.Feature("notes.txt"); ok {
		t.Error("a plain file under specs became a feature")
	}
}
