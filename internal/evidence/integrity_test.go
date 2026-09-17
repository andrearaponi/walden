package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

func integrityRecord(t *testing.T, execution string) Record {
	t.Helper()
	text := `{"task_fingerprint":"sha256:task","requirements_fingerprint":"sha256:req","design_fingerprint":"sha256:des","tasks_fingerprint":"sha256:plan","code_identity":"sha256:code","result":"passed","verified_at":"1999-01-01T00:00:00Z","steps":[],"task_fingerprint_scheme":"walden/task-definition/v2"`
	if execution != "" {
		text += `,"execution":` + execution
	}
	var record Record
	if err := json.Unmarshal([]byte(text+`}`), &record); err != nil {
		t.Fatal(err)
	}
	return record
}

const integrityPureFacts = `{"origin":"verify","policy":"verify-purity/v1","assertion_result":"passed","integrity":"pure","before_code_identity":"sha256:code","after_code_identity":"sha256:code"}`

func TestEvidenceIntegrityLedgerSemantics(t *testing.T) {
	chain := ChainFingerprints{Requirements: "sha256:req", Design: "sha256:des"}
	leafs := []LeafTask{{ID: "1", Completed: true, Fingerprint: "sha256:task"}}
	for _, tc := range []struct{ name, facts, state string }{
		{"supported pure record", integrityPureFacts, "verified"},
		{"missing provenance", "", "unattested"},
		{"unknown policy", strings.Replace(integrityPureFacts, "verify-purity/v1", "unknown/v9", 1), "unattested"},
		{"contaminated passing command", strings.Replace(integrityPureFacts, `"pure"`, `"contaminated"`, 1), "failed"},
		{"inconsistent pure identities", strings.Replace(integrityPureFacts, `"after_code_identity":"sha256:code"`, `"after_code_identity":"sha256:other"`, 1), "failed"},
		{"missing capture", strings.Replace(integrityPureFacts, `"after_code_identity":"sha256:code"`, `"after_code_identity":""`, 1), "unattested"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := Document{Tasks: map[string]Record{"1": integrityRecord(t, tc.facts)}}
			entry := Derive(document, chain, "sha256:code", true, leafs)[0]
			if entry.State != tc.state {
				t.Fatalf("state = %s, want %s", entry.State, tc.state)
			}
		})
	}
	t.Run("absence is not identity equality", func(t *testing.T) {
		record := integrityRecord(t, "")
		record.CodeIdentity = ""
		got := Derive(Document{Tasks: map[string]Record{"1": record}}, chain, "", false, leafs)[0]
		if got.State == "verified" {
			t.Fatal("two missing identities became a verified guarantee")
		}
	})
	t.Run("missing bindings are unknown rather than known changes", func(t *testing.T) {
		for _, field := range []string{"requirements", "task"} {
			record := integrityRecord(t, integrityPureFacts)
			if field == "requirements" {
				record.RequirementsFingerprint = ""
			} else {
				record.TaskFingerprint = ""
			}
			entry := Derive(Document{Tasks: map[string]Record{"1": record}}, chain, "sha256:code", true, leafs)[0]
			if entry.State != "unattested" {
				t.Errorf("missing %s was reported as a known contract change: %+v", field, entry)
			}
		}
	})
	t.Run("failed facts dominate gaps", func(t *testing.T) {
		record := integrityRecord(t, "")
		record.Result = ResultFailed
		record.RequirementsFingerprint = "changed"
		if got := Derive(Document{Tasks: map[string]Record{"1": record}}, chain, "different", true, leafs)[0]; got.State != "failed" {
			t.Fatalf("failure was grandfathered in: %+v", got)
		}
	})
	t.Run("mixed ledger preserves old facts", func(t *testing.T) {
		root := t.TempDir()
		old := integrityRecord(t, "")
		document := Document{SchemaVersion: "v1alpha1", Feature: "f", Tasks: map[string]Record{"1": old}}
		path := DocumentPath(root, "f")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(document)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(root, "f")
		if err != nil {
			t.Fatal(err)
		}
		untouched, _ := os.ReadFile(path)
		if string(untouched) != string(data) {
			t.Fatal("reading migrated the ledger")
		}
		loaded.Tasks["2"] = integrityRecord(t, integrityPureFacts)
		if err := Save(root, loaded); err != nil {
			t.Fatal(err)
		}
		mixed, err := Load(root, "f")
		if err != nil {
			t.Fatal(err)
		}
		if mixed.SchemaVersion != "v1alpha2" || !reflect.DeepEqual(mixed.Tasks["1"], old) {
			t.Fatalf("mixed ledger changed old facts or missed new format: %+v", mixed)
		}
		for _, version := range []string{"", "v1alpha1", "v1alpha2"} {
			document.SchemaVersion = version
			data, _ := json.Marshal(document)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root, "f"); err != nil {
				t.Errorf("supported format %q: %v", version, err)
			}
		}
		document.SchemaVersion = "v9alpha9"
		data, _ = json.Marshal(document)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root, "f"); err == nil || strings.Contains(err.Error(), "remove the file") {
			t.Fatalf("unsupported schema did not preserve the evidence for diagnosis: %v", err)
		}
	})
}

type integrityGitFailure struct{ unborn bool }

func (r integrityGitFailure) Run(_ context.Context, _ string, args ...string) (shell.Response, error) {
	for _, arg := range args {
		switch arg {
		case "status":
			return shell.Response{}, nil
		case "ls-tree":
			return shell.Response{ExitCode: 128, Stderr: "tree unavailable"}, nil
		case "rev-parse":
			if r.unborn {
				return shell.Response{ExitCode: 1}, nil
			}
			return shell.Response{Stdout: strings.Repeat("a", 40) + "\n"}, nil
		case "symbolic-ref":
			return shell.Response{Stdout: "refs/heads/main\n"}, nil
		case "show-ref":
			if r.unborn {
				return shell.Response{ExitCode: 1}, nil
			}
			return shell.Response{}, nil
		}
	}
	return shell.Response{}, errors.New("unexpected git invocation")
}

func TestEvidenceIntegrityIdentityCaptureFailures(t *testing.T) {
	if manifest, ok := CaptureManifest(context.Background(), integrityGitFailure{}, t.TempDir()); ok {
		t.Fatalf("an unreadable existing commit yielded a usable empty tree: %v", manifest)
	}
	if manifest, ok := CaptureManifest(context.Background(), integrityGitFailure{unborn: true}, t.TempDir()); !ok || manifest == nil {
		t.Fatal("a positively identified unborn repository lost its valid overlay")
	}
}
