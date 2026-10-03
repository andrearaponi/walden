package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func adoptDiagnosticRun(args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func adoptDiagnosticJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("invalid envelope: %v\n%s", err, text)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected one JSON envelope followed by EOF, got %v: %s", err, text)
	}
	if value["schema_version"] != "v0beta1" || value["command"] != "adopt" {
		t.Fatalf("unexpected envelope: %+v", value)
	}
	return value
}

func adoptDiagnosticObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %#v", value)
	}
	return object
}

func adoptDiagnosticFeatures(t *testing.T, adoption map[string]any) map[string]map[string]any {
	t.Helper()
	values, ok := adoption["features"].([]any)
	if !ok {
		t.Fatalf("missing feature array: %+v", adoption)
	}
	features := map[string]map[string]any{}
	for _, value := range values {
		feature := adoptDiagnosticObject(t, value)
		features[feature["feature"].(string)] = feature
	}
	return features
}

func adoptDiagnosticClone(t *testing.T, root, source, target string, change func(string, string) string) {
	t.Helper()
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(root, ".walden", "specs", source, name))
		if err != nil {
			t.Fatal(err)
		}
		writeRawFeatureFile(t, root, target, name, change(name, string(data)))
	}
}

func TestAdoptDiagnosticsPlanOutput(t *testing.T) {
	root := adoptFixture(t)
	adoptDiagnosticClone(t, root, "old-era", "empty", func(name, text string) string {
		if name == "tasks.md" {
			return strings.ReplaceAll(text, "[x]", "[ ]")
		}
		return text
	})
	adoptDiagnosticClone(t, root, "old-era", "bad", func(name, text string) string {
		if name == "tasks.md" {
			return strings.Replace(text, `["go", "test", "./..."]`, `[not-json]`, 1)
		}
		return text
	})

	stdout, stderr, code := adoptDiagnosticRun("adopt", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("plan exit=%d stderr=%s", code, stderr)
	}
	envelope := adoptDiagnosticJSON(t, stdout)
	result := adoptDiagnosticObject(t, envelope["result"])
	adoption := adoptDiagnosticObject(t, result["adoption"])
	work := adoptDiagnosticObject(t, adoption["workload"])
	if work["assessed_tasks"] != float64(1) || work["assessed_steps"] != float64(1) {
		t.Fatalf("wrong assessed workload: %+v", work)
	}
	unknown, ok := work["unassessed_features"].([]any)
	if !ok || len(unknown) != 1 || unknown[0] != "bad" {
		t.Fatalf("unknown workload silently counted: %+v", work)
	}
	features := adoptDiagnosticFeatures(t, adoption)
	for name, count := range map[string]float64{"old-era": 1, "empty": 0} {
		feature := features[name]
		w := adoptDiagnosticObject(t, feature["workload"])
		if w["available"] != true || w["tasks"] != count || w["steps"] != count {
			t.Fatalf("%s workload = %+v", name, w)
		}
		if feature["class"] != "backfill" {
			t.Fatalf("changed existing class for %s: %+v", name, feature)
		}
	}
	bad := adoptDiagnosticObject(t, features["bad"]["workload"])
	if bad["available"] != false || bad["reason"] == "" || bad["reason"] == nil {
		t.Fatalf("unavailable workload lost its reason: %+v", bad)
	}
	for _, key := range []string{"tasks", "steps"} {
		if _, exists := bad[key]; exists {
			t.Fatalf("unavailable workload contains %s: %+v", key, bad)
		}
	}
	if features["old-era"]["reprove_count"] != float64(1) || features["bad"]["class"] != "blocked" {
		t.Fatalf("existing fields changed: %+v", features)
	}
	if strings.Contains(stdout, `"elapsed_ms"`) || adoption["apply"] != false {
		t.Fatalf("planning invented execution: %s", stdout)
	}

	text, stderr, code := adoptDiagnosticRun("adopt")
	if code != 0 || stderr != "" {
		t.Fatalf("text plan exit=%d stderr=%s", code, stderr)
	}
	for _, fragment := range []string{"Assessed workload: 1 task(s), 1 declared step(s)", "unassessed: bad", "workload unavailable:", "0 task(s), 0 declared step(s)"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("text plan missing %q:\n%s", fragment, text)
		}
	}

	stdout, _, code = adoptDiagnosticRun("adopt", "old-era", "--json")
	if code != 0 {
		t.Fatalf("named plan failed: %s", stdout)
	}
	result = adoptDiagnosticObject(t, adoptDiagnosticJSON(t, stdout)["result"])
	adoption = adoptDiagnosticObject(t, result["adoption"])
	if len(adoptDiagnosticFeatures(t, adoption)) != 1 || adoptDiagnosticObject(t, adoption["scope"])["kind"] != "feature" {
		t.Fatalf("named plan expanded: %+v", adoption)
	}
	work = adoptDiagnosticObject(t, adoption["workload"])
	if unknown, ok := work["unassessed_features"].([]any); !ok || len(unknown) != 0 {
		t.Fatalf("named plan includes unrelated unknown work: %+v", work)
	}
}
