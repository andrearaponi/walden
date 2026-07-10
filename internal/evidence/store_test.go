package evidence

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleRecord() Record {
	return Record{
		TaskFingerprint:         "sha256:task",
		RequirementsFingerprint: "sha256:req",
		DesignFingerprint:       "sha256:des",
		TasksFingerprint:        "sha256:tasks",
		CodeIdentity:            "sha256:code",
		Steps: []StepResult{
			{Command: []string{"go", "test", "./..."}, ExpectedExit: 0, ActualExit: 0, OutputDigest: "sha256:out"},
		},
		Result:     ResultPassed,
		VerifiedAt: "2026-07-10T18:00:00Z",
	}
}

func TestEvidenceStoreAbsentFileIsEmptyLedger(t *testing.T) {
	document, err := Load(t.TempDir(), "user-auth")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if document.SchemaVersion != SchemaVersion || document.Feature != "user-auth" {
		t.Fatalf("empty ledger = %+v, want stamped schema and feature", document)
	}
	if len(document.Tasks) != 0 {
		t.Fatalf("empty ledger carries %d tasks", len(document.Tasks))
	}
}

func TestEvidenceStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	saved := Document{Feature: "user-auth", Tasks: map[string]Record{"1.1": sampleRecord()}}

	if err := Save(root, saved); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(root, "user-auth")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", loaded.SchemaVersion, SchemaVersion)
	}
	if !reflect.DeepEqual(loaded.Tasks["1.1"], sampleRecord()) {
		t.Fatalf("round-trip mismatch:\n%+v\nwant\n%+v", loaded.Tasks["1.1"], sampleRecord())
	}
}

func TestEvidenceStoreReplacesEntryPerTask(t *testing.T) {
	root := t.TempDir()
	first := sampleRecord()
	if err := Save(root, Document{Feature: "f", Tasks: map[string]Record{"1.1": first}}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	replacement := sampleRecord()
	replacement.CodeIdentity = "sha256:new"
	if err := Save(root, Document{Feature: "f", Tasks: map[string]Record{"1.1": replacement}}); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, err := Load(root, "f")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks["1.1"].CodeIdentity != "sha256:new" {
		t.Fatalf("entry not replaced: %+v", loaded.Tasks)
	}
}

func TestEvidenceStoreWritesAtomicallyInsideWalden(t *testing.T) {
	root := t.TempDir()
	if err := Save(root, Document{Feature: "f", Tasks: map[string]Record{"1.1": sampleRecord()}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := DocumentPath(root, "f")
	if !strings.HasPrefix(path, filepath.Join(root, ".walden", "evidence")) {
		t.Fatalf("evidence path %q escapes .walden/evidence", path)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read evidence dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".walden-doc-") {
			t.Fatalf("staging leftover %s", entry.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("evidence dir holds %d entries, want 1", len(entries))
	}
}
