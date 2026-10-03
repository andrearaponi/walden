package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

func adoptDiagnosticFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		digest := sha256.Sum256(data)
		files[filepath.ToSlash(relative)] = hex.EncodeToString(digest[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func adoptDiagnosticKeys(t *testing.T, object map[string]any, expected ...string) {
	t.Helper()
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sort.Strings(expected)
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("storage fields changed: got %v want %v", keys, expected)
	}
}

func adoptCompatibilityEnvelope(t *testing.T, text string) output.Envelope {
	t.Helper()
	var envelope output.Envelope
	decoder := json.NewDecoder(strings.NewReader(text))
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("decode: %v\n%s", err, text)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("not a single envelope: %v", err)
	}
	return envelope
}

func TestAdoptDiagnosticsPersistenceAndCompatibility(t *testing.T) {
	t.Run("storage retry noop and ordinary read/verify surfaces", func(t *testing.T) {
		root := adoptFixture(t)
		adoptDiagnosticTasks(t, root, "old-era", adoptDiagnosticTask("1", ""))
		before := adoptDiagnosticFiles(t, root)
		const sentinel = "RAW-OUTPUT-NOT-PRESENT-IN-DECLARED-ARGV"
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{ExitCode: 1, Stderr: sentinel}))
		stdout, _, code := adoptDiagnosticRun("adopt", "old-era", "--apply", "--json")
		if code != 1 || !strings.Contains(stdout, sentinel) {
			t.Fatalf("failure diagnostic not available to the caller: %d %s", code, stdout)
		}
		path := evidence.DocumentPath(root, "old-era")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(sentinel)) || bytes.Contains(data, []byte("elapsed")) {
			t.Fatalf("diagnostics persisted raw output or timing: %s", data)
		}
		var stored map[string]any
		if err := json.Unmarshal(data, &stored); err != nil {
			t.Fatal(err)
		}
		adoptDiagnosticKeys(t, stored, "schema_version", "feature", "tasks")
		if stored["schema_version"] != "v1alpha2" {
			t.Fatalf("storage schema changed: %v", stored["schema_version"])
		}
		record := adoptDiagnosticObject(t, adoptDiagnosticObject(t, stored["tasks"])["1"])
		adoptDiagnosticKeys(t, record, "task_fingerprint", "task_fingerprint_scheme", "execution", "requirements_fingerprint", "design_fingerprint", "tasks_fingerprint", "code_identity", "profile", "steps", "result", "verified_at")
		step := adoptDiagnosticObject(t, record["steps"].([]any)[0])
		adoptDiagnosticKeys(t, step, "command", "expected_exit", "actual_exit", "output_digest")
		adoptDiagnosticKeys(t, adoptDiagnosticObject(t, record["execution"]), "origin", "policy", "assertion_result", "integrity", "before_code_identity", "after_code_identity")
		after := adoptDiagnosticFiles(t, root)
		for name, digest := range after {
			if before[name] != digest && name != ".walden/evidence/old-era.json" && !strings.HasPrefix(name, ".walden/specs/old-era/") {
				t.Errorf("unexpected artifact/change: %s", name)
			}
		}
		for name := range before {
			if _, exists := after[name]; !exists {
				t.Errorf("reporting removed %s", name)
			}
		}

		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{}))
		if stdout, _, code = adoptDiagnosticRun("adopt", "old-era", "--apply", "--json"); code != 0 {
			t.Fatalf("retry failed: %s", stdout)
		}
		beforeNoop, _ := os.ReadFile(path)
		idle := overrideCommandRunner(t, testutil.NewFakeRunner())
		stdout, _, code = adoptDiagnosticRun("adopt", "old-era", "--apply", "--json")
		afterNoop, _ := os.ReadFile(path)
		if code != 0 || len(idle.Calls()) != 0 || !bytes.Equal(beforeNoop, afterNoop) {
			t.Fatalf("no-op rewrote/replayed: %d %s", code, stdout)
		}
		feature := adoptCompatibilityEnvelope(t, stdout).Result.Adoption.Features[0]
		if len(feature.Evidence) != 0 || feature.EvidencePersisted != nil {
			t.Fatalf("historical evidence became a new execution: %+v", feature)
		}
		for _, args := range [][]string{
			{"status", "old-era"}, {"task", "status", "old-era"}, {"evidence", "status", "old-era"}, {"release", "check", "old-era"},
		} {
			for _, jsonMode := range []bool{false, true} {
				command := append([]string{}, args...)
				if jsonMode {
					command = append(command, "--json")
				}
				out, errText, _ := adoptDiagnosticRun(command...)
				for _, forbidden := range []string{"elapsed_ms", "elapsed=", "assertion=", "evidence_persisted", "Assessed workload:"} {
					if strings.Contains(out+errText, forbidden) {
						t.Errorf("adoption details leaked into %v: %s %s", command, out, errText)
					}
				}
				if jsonMode {
					adoptCompatibilityEnvelope(t, out)
				}
			}
		}
		for _, check := range []bool{true, false} {
			overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{}))
			args := []string{"verify", "old-era", "--all", "--json"}
			if check {
				args = append(args, "--check")
			}
			before, _ := os.ReadFile(path)
			out, _, code := adoptDiagnosticRun(args...)
			after, _ := os.ReadFile(path)
			envelope := adoptCompatibilityEnvelope(t, out)
			if code != 0 || len(envelope.Result.Evidence) != 1 || envelope.Result.Evidence[0].ElapsedMS != nil || strings.Contains(out, "evidence_persisted") || (check && !bytes.Equal(before, after)) {
				t.Fatalf("ordinary verify changed: %v %d %s", args, code, out)
			}
		}
	})

	t.Run("completion remains generator-capable and untimed", func(t *testing.T) {
		root := adoptFixture(t)
		adoptDiagnosticTasks(t, root, "old-era", strings.Replace(adoptDiagnosticTask("1", ""), "[x]", "[ ]", 1))
		idle := overrideCommandRunner(t, testutil.NewFakeRunner())
		if out, _, code := adoptDiagnosticRun("adopt", "old-era", "--apply", "--json"); code != 0 || len(idle.Calls()) != 0 {
			t.Fatalf("pending-task backfill executed work: %d %s", code, out)
		}
		adoptDiagnosticOverrideRunner(t, adoptDiagnosticRunnerFunc(func(context.Context, string, ...string) (shell.Response, error) {
			if err := os.WriteFile(filepath.Join(root, "generated.txt"), []byte("output"), 0o644); err != nil {
				t.Fatal(err)
			}
			return shell.Response{}, nil
		}))
		out, _, code := adoptDiagnosticRun("task", "complete", "old-era", "1", "--json")
		if code != 0 || strings.Contains(out, "elapsed_ms") || strings.Contains(out, "evidence_persisted") {
			t.Fatalf("completion policy/output changed: %d %s", code, out)
		}
		adoptCompatibilityEnvelope(t, out)
		ledger, err := evidence.Load(root, "old-era")
		if err != nil {
			t.Fatal(err)
		}
		facts := ledger.Tasks["1"].Execution
		if facts == nil || facts.Origin != "complete" || facts.Integrity != "post-state" || facts.AfterCodeIdentity == "" {
			t.Fatalf("completion no longer binds its post-state: %+v", facts)
		}
	})

	t.Run("shipped demo remains a read-only compatibility fixture", func(t *testing.T) {
		root := filepath.Join(authoringSourceRoot(t), "examples/todo-app-demo")
		before := adoptDiagnosticFiles(t, root)
		t.Chdir(root)
		out, errText, code := adoptDiagnosticRun("validate", "todo-app", "--all", "--json")
		if code != 0 {
			t.Fatalf("demo validation regressed: %s %s", out, errText)
		}
		adoptCompatibilityEnvelope(t, out)
		out, _, code = adoptDiagnosticRun("status", "todo-app", "--json")
		envelope := adoptCompatibilityEnvelope(t, out)
		if code != 0 || len(envelope.Result.Documents) != 3 {
			t.Fatalf("demo status regressed: %s", out)
		}
		for _, document := range envelope.Result.Documents {
			if !document.Fresh || document.Status != "approved" {
				t.Fatalf("demo chain no longer intact: %+v", document)
			}
		}
		if !reflect.DeepEqual(before, adoptDiagnosticFiles(t, root)) {
			t.Fatal("demo smoke wrote into the compatibility fixture")
		}
	})
}
