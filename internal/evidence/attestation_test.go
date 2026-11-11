package evidence

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

func captureOf(t *testing.T, root string) Capture {
	t.Helper()
	capture, ok := CaptureTree(context.Background(), shell.NewExecRunner(), root)
	if !ok {
		t.Fatalf("the tree of %s cannot be captured", root)
	}
	return capture
}

// TestCaptureCommitted: a capture is committed when the working tree holds
// what its commit holds, outside .walden/ and the ignored files.
func TestCaptureCommitted(t *testing.T) {
	t.Run("committed", func(t *testing.T) {
		root, head := committedRepo(t)
		if capture := captureOf(t, root); !capture.Committed() || capture.Commit != head {
			t.Fatalf("a clean tree: committed=%t at %q, want %s", capture.Committed(), capture.Commit, head)
		}

		// committedRepo ignores *.log.
		writeTreeFile(t, root, "build.log", "noise\n", 0o644)
		writeTreeFile(t, root, ".walden/evidence/feature.json", "{}\n", 0o644)
		if capture := captureOf(t, root); !capture.Committed() {
			t.Errorf("an ignored file and a change under .walden/ make the capture uncommitted: %v", capture.Differing)
		}

		// A path staged and then restored: the index differs from the
		// commit, the working tree does not. The proof reads the tree.
		writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
		historyGit(t, root, "add", "a.txt")
		writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)
		if status := historyGit(t, root, "status", "--porcelain", "--", "a.txt"); status == "" {
			t.Fatal("the fixture holds no index-only change")
		}
		if capture := captureOf(t, root); !capture.Committed() {
			t.Errorf("a path staged and restored makes the capture uncommitted: %v", capture.Differing)
		}
	})

	for name, change := range map[string]func(t *testing.T, root string){
		"modified":  func(t *testing.T, root string) { writeTreeFile(t, root, "a.txt", "changed\n", 0o644) },
		"untracked": func(t *testing.T, root string) { writeTreeFile(t, root, "new.txt", "new\n", 0o644) },
		"deleted": func(t *testing.T, root string) {
			if err := os.Remove(root + "/b.txt"); err != nil {
				t.Fatal(err)
			}
		},
		"staged": func(t *testing.T, root string) {
			writeTreeFile(t, root, "a.txt", "changed\n", 0o644)
			historyGit(t, root, "add", "a.txt")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, _ := committedRepo(t)
			change(t, root)
			if capture := captureOf(t, root); capture.Committed() {
				t.Errorf("a tree with a %s path is committed", name)
			}
		})
	}

	t.Run("no commit", func(t *testing.T) {
		root := isolatedRepo(t)
		writeTreeFile(t, root, "a.txt", "alpha\n", 0o644)
		if capture := captureOf(t, root); capture.Committed() || capture.Commit != "" {
			t.Errorf("a repository without a commit: committed=%t at %q", capture.Committed(), capture.Commit)
		}
		if (Capture{}).Committed() {
			t.Error("the zero capture is committed")
		}
	})
}

func attestedSample() Record {
	record := sampleRecord()
	record.TaskFingerprint = "sha256:declared"
	record.Statements = &StatementFacts{Scheme: spec.StatementScheme, Fingerprints: map[string]string{"R1.AC1": "sha256:statement"}}
	return record
}

// TestAttest: an attestation is the fact that a pass ran on a committed
// capture, with the definition and the statements it attests.
func TestAttest(t *testing.T) {
	committed := Capture{Commit: "0123456789abcdef0123456789abcdef01234567", Manifest: Manifest{"a.txt": "100644:aaa"}}
	record := attestedSample()

	attestation := Attest(record, committed)
	if attestation == nil {
		t.Fatal("a pass on a committed capture gets no attestation")
	}
	want := Attestation{Commit: committed.Commit, TaskFingerprint: "sha256:declared", Statements: *record.Statements}
	if !reflect.DeepEqual(*attestation, want) {
		t.Errorf("attestation = %+v, want %+v", *attestation, want)
	}
	if attestation.Sealed {
		t.Error("an attestation written by an execution is marked as sealed")
	}
	// The attestation keeps its own copy: a later change of the record's
	// statement facts does not reach it.
	record.Statements.Fingerprints["R1.AC1"] = "sha256:other"
	if attestation.Statements.Fingerprints["R1.AC1"] != "sha256:statement" {
		t.Error("the attestation shares the fingerprints of the record")
	}

	differing := committed
	differing.Differing = map[string]string{"a.txt": "100644:bbb"}
	failed := attestedSample()
	failed.Result = ResultFailed
	withoutStatements := attestedSample()
	withoutStatements.Statements = nil
	for name, tc := range map[string]struct {
		record  Record
		capture Capture
	}{
		"a differing path":   {attestedSample(), differing},
		"no commit":          {attestedSample(), Capture{Manifest: committed.Manifest}},
		"no capture":         {attestedSample(), Capture{}},
		"a failed result":    {failed, committed},
		"no statement facts": {withoutStatements, committed},
	} {
		if got := Attest(tc.record, tc.capture); got != nil {
			t.Errorf("%s: attested %+v", name, *got)
		}
	}
}

// TestNamedStatements: the facts are those a record stores today, the
// fingerprint of each statement block the task names.
func TestNamedStatements(t *testing.T) {
	const body = "# Requirements Document\n\n## Requirements\n\n### R1 First\n\n#### Acceptance Criteria\n\n" +
		"1. `R1.AC1` WHEN the first event occurs, the system SHALL respond.\n   - Acceptance check: the response is visible.\n" +
		"2. `R1.AC2` WHEN the second event occurs, the system SHALL respond.\n\n" +
		"## Non-Functional Requirements\n\n- `NFR1` The system SHALL stay deterministic.\n"
	blocks := spec.StatementBlocks(body)
	if len(blocks) != 3 {
		t.Fatalf("the fixture defines %d statements, want three", len(blocks))
	}
	fingerprints := spec.StatementFingerprints(body)

	proof := spec.VerificationSpec{Steps: []spec.VerificationStep{
		{Argv: []string{"true"}, Covers: []string{"R1.AC2"}},
		{Argv: []string{"true"}, Covers: []string{"NFR1", "R1.AC2"}},
	}}
	// R1 is a requirement heading: it defines no statement and gets no entry.
	facts := NamedStatements(fingerprints, []string{"R1", "R1.AC1"}, proof)
	if facts == nil || facts.Scheme != spec.StatementScheme {
		t.Fatalf("facts = %+v, want the scheme %s", facts, spec.StatementScheme)
	}
	want := map[string]string{
		"R1.AC1": spec.StatementFingerprint(blocks["R1.AC1"]),
		"R1.AC2": spec.StatementFingerprint(blocks["R1.AC2"]),
		"NFR1":   spec.StatementFingerprint(blocks["NFR1"]),
	}
	if !reflect.DeepEqual(facts.Fingerprints, want) {
		t.Errorf("fingerprints = %v, want %v", facts.Fingerprints, want)
	}

	// A task that names no statement still records the scheme with an empty
	// set, so a reader can tell it from a record that kept no facts.
	none := NamedStatements(fingerprints, []string{"R1"}, spec.VerificationSpec{LegacyCommand: "true"})
	if none == nil || none.Scheme != spec.StatementScheme || none.Fingerprints == nil || len(none.Fingerprints) != 0 {
		t.Errorf("a task that names no statement: %+v", none)
	}
}

func attestedLedger() Document {
	attested := attestedSample()
	attested.Attestation = &Attestation{
		Commit: "0123456789abcdef0123456789abcdef01234567", TaskFingerprint: "sha256:declared",
		Statements: *attested.Statements,
	}
	return Document{Feature: "f", Tasks: map[string]Record{"1": sampleRecord(), "2": attested}}
}

func savedLedger(t *testing.T, document Document) (root string, data []byte, version string) {
	t.Helper()
	root = t.TempDir()
	if err := Save(root, document); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(DocumentPath(root, document.Feature))
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	return root, data, header.SchemaVersion
}

// TestLedgerVersionFollowsContent: a ledger that holds an attestation is
// written as v1alpha3, any other as v1alpha2, whatever it was before.
func TestLedgerVersionFollowsContent(t *testing.T) {
	plain := Document{Feature: "f", Tasks: map[string]Record{"1": sampleRecord()}}
	if _, _, version := savedLedger(t, plain); version != "v1alpha2" {
		t.Errorf("a ledger without attestations is written as %s, want v1alpha2", version)
	}

	root, _, version := savedLedger(t, attestedLedger())
	if version != "v1alpha3" {
		t.Fatalf("a ledger with an attestation is written as %s, want v1alpha3", version)
	}
	loaded, err := Load(root, "f")
	if err != nil {
		t.Fatalf("this binary cannot read the ledger it wrote: %v", err)
	}
	if !reflect.DeepEqual(loaded.Tasks, attestedLedger().Tasks) {
		t.Errorf("round trip changed the records:\n%+v\nwant\n%+v", loaded.Tasks, attestedLedger().Tasks)
	}

	// Once its last attestation is gone the ledger is v1alpha2 again.
	record := loaded.Tasks["2"]
	record.Attestation = nil
	loaded.Tasks["2"] = record
	if _, _, version := savedLedger(t, loaded); version != "v1alpha2" {
		t.Errorf("a ledger whose last attestation is gone is written as %s, want v1alpha2", version)
	}

	// The version a document carries is not trusted in either direction.
	stamped := plain
	stamped.SchemaVersion = "v1alpha3"
	if _, _, version := savedLedger(t, stamped); version != "v1alpha2" {
		t.Errorf("a document stamped v1alpha3 without attestations is written as %s", version)
	}
	_, data, _ := savedLedger(t, attestedLedger())
	relabelled := []byte(strings.Replace(string(data), `"schema_version": "v1alpha3"`, `"schema_version": "v1alpha2"`, 1))
	decoded, err := Decode(relabelled, "f")
	if err != nil || decoded.Tasks["2"].Attestation == nil {
		t.Fatalf("an attestation in a ledger labelled v1alpha2: %v, %+v", err, decoded.Tasks["2"].Attestation)
	}
	if _, _, version := savedLedger(t, decoded); version != "v1alpha3" {
		t.Errorf("that ledger is written back as %s, want v1alpha3", version)
	}

	// A later version is refused, naming it and the two this binary writes.
	later := []byte(strings.Replace(string(data), "v1alpha3", "v1alpha4", 1))
	_, err = Decode(later, "f")
	if err == nil {
		t.Fatal("a ledger of a later version is accepted")
	}
	for _, want := range []string{"v1alpha4", "v1alpha2", "v1alpha3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not name %s", err, want)
		}
	}
}

// TestFrozenDecoderRefusesAttestedLedger: the release before proof kinds
// refuses a ledger that holds an attestation, naming its version, and reads
// the ledger this binary writes when there is none.
func TestFrozenDecoderRefusesAttestedLedger(t *testing.T) {
	_, attested, _ := savedLedger(t, attestedLedger())
	_, err := frozenV0145Decode(attested, "f")
	if err == nil {
		t.Fatal("v0.14.5 reads a ledger that holds an attestation: it would prove the delivery again")
	}
	for _, want := range []string{"schema v1alpha3 is unsupported", "use a compatible walden reader"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal of v0.14.5 %q does not hold %q", err, want)
		}
	}

	plain := Document{Feature: "f", Tasks: map[string]Record{"1": sampleRecord(), "2": attestedSample()}}
	_, written, _ := savedLedger(t, plain)
	document, err := frozenV0145Decode(written, "f")
	if err != nil {
		t.Fatalf("v0.14.5 refuses a ledger without attestations: %v", err)
	}
	if len(document.Tasks) != 2 {
		t.Errorf("v0.14.5 reads %d record(s), want two", len(document.Tasks))
	}

	// The same ledger is readable again once the attestation is removed.
	ledger := attestedLedger()
	record := ledger.Tasks["2"]
	record.Attestation = nil
	ledger.Tasks["2"] = record
	_, removed, _ := savedLedger(t, ledger)
	if _, err := frozenV0145Decode(removed, "f"); err != nil {
		t.Errorf("v0.14.5 refuses the ledger after its attestation is removed: %v", err)
	}
}
