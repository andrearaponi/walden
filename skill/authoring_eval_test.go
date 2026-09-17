package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var authoringBundleFiles = []string{
	"skill/walden/SKILL.md",
	"templates/spec/requirements.md.tmpl",
	"templates/spec/design.md.tmpl",
	"templates/spec/tasks.md.tmpl",
}

type authoringScenario struct {
	ID        string   `json:"id"`
	Seed      string   `json:"seed"`
	Prompt    string   `json:"prompt"`
	FollowUps []string `json:"follow_ups"`
	Criteria  []string `json:"criteria"`
	Expect    []string `json:"expect"`
}

type authoringCheck struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Anchor  string `json:"anchor"`
}

type authoringRun struct {
	Scenario   string           `json:"scenario"`
	Variant    string           `json:"variant"`
	Verdict    string           `json:"verdict"`
	Transcript string           `json:"transcript"`
	Artifact   string           `json:"artifact"`
	Checks     []authoringCheck `json:"checks"`
}

type authoringReport struct {
	Kind           string            `json:"kind"`
	BaselineCommit string            `json:"baseline_commit"`
	Agent          string            `json:"agent"`
	AgentVersion   string            `json:"agent_version"`
	Model          string            `json:"model"`
	Settings       string            `json:"settings"`
	Baseline       map[string]string `json:"baseline"`
	Candidate      map[string]string `json:"candidate"`
	Runs           []authoringRun    `json:"runs"`
}

func authoringRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readAuthoringScenarios(t *testing.T) []authoringScenario {
	t.Helper()
	data, err := os.ReadFile("testdata/authoring-eval/scenarios.json")
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []authoringScenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		t.Fatal(err)
	}
	return scenarios
}

func bundleHashes(t *testing.T) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	for _, name := range authoringBundleFiles {
		data, err := os.ReadFile(filepath.Join(authoringRoot(t), name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	return hashes
}

func TestAuthoringEvalFixtures(t *testing.T) {
	scenarios := readAuthoringScenarios(t)
	want := map[string]bool{"bugfix": true, "refactoring": true, "small-feature": true, "contract-change": true, "ambiguous": true}
	criteria := map[string]bool{}
	for _, scenario := range scenarios {
		if !want[scenario.ID] {
			t.Fatalf("unexpected or duplicate scenario %q", scenario.ID)
		}
		delete(want, scenario.ID)
		if scenario.Prompt == "" || scenario.Seed == "" || len(scenario.Criteria) == 0 || len(scenario.Expect) == 0 {
			t.Fatalf("incomplete scenario %s", scenario.ID)
		}
		for _, id := range scenario.Criteria {
			criteria[id] = true
		}
	}
	if len(want) != 0 || len(criteria) != 12 {
		t.Fatalf("missing scenarios %v or incomplete AC inventory %v", want, criteria)
	}
	for _, name := range []string{"README.md", "seed/greet.sh", "seed/test_greet.sh", "seed/requirements.md", "seed/design.md", "seed/tasks.md", "seed/constitution.md"} {
		data, err := os.ReadFile(filepath.Join("testdata/authoring-eval", name))
		if err != nil || len(data) == 0 {
			t.Fatalf("missing/nonempty fixture %s: %v", name, err)
		}
	}
}

// This is a test-only integrity check over reviewed observations, not an
// agent runner, an independent transcript judge, or a certification API.
func checkAuthoringReport(report authoringReport, scenarios []authoringScenario, current map[string]string, dir string, observed bool) error {
	if observed && report.Kind != "observed" {
		return fmt.Errorf("real acceptance requires observed runs, not synthetic fixtures")
	}
	if report.BaselineCommit != "3995cab96c79f2de891f7c86ed559d48d3a7253a" || report.Agent == "" || report.AgentVersion == "" || report.Model == "" || report.Settings == "" {
		return fmt.Errorf("missing baseline or agent/model/settings provenance")
	}
	for name, hash := range current {
		if report.Candidate[name] != hash {
			return fmt.Errorf("candidate content is stale: %s", name)
		}
		baseline, err := hex.DecodeString(report.Baseline[name])
		if err != nil || len(baseline) != sha256.Size {
			return fmt.Errorf("missing baseline content identity: %s", name)
		}
	}
	catalog := map[string]authoringScenario{}
	for _, scenario := range scenarios {
		catalog[scenario.ID] = scenario
	}
	seen := map[string]bool{}
	for _, run := range report.Runs {
		scenario, exists := catalog[run.Scenario]
		key := run.Variant + ":" + run.Scenario
		if !exists || (run.Variant != "baseline" && run.Variant != "candidate") || seen[key] {
			return fmt.Errorf("unexpected or duplicate run %s", key)
		}
		seen[key] = true
		if run.Verdict != "pass" && run.Verdict != "fail" {
			return fmt.Errorf("run %s was not exercised", key)
		}
		if run.Variant == "candidate" && run.Verdict != "pass" {
			return fmt.Errorf("candidate run %s failed", key)
		}
		for _, anchor := range []string{run.Transcript, run.Artifact} {
			if err := checkAuthoringAnchor(dir, anchor); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
		checks := map[string]bool{}
		for _, check := range run.Checks {
			if checks[check.ID] || !containsAuthoringID(scenario.Criteria, check.ID) {
				return fmt.Errorf("unexpected or duplicate criterion %s in %s", check.ID, key)
			}
			checks[check.ID] = true
			if (check.Verdict != "pass" && check.Verdict != "fail") || (run.Variant == "candidate" && check.Verdict != "pass") || (run.Verdict == "pass" && check.Verdict != "pass") {
				return fmt.Errorf("criterion %s is not satisfied in %s", check.ID, key)
			}
			if err := checkAuthoringAnchor(dir, check.Anchor); err != nil {
				return fmt.Errorf("%s/%s: %w", key, check.ID, err)
			}
		}
		if len(checks) != len(scenario.Criteria) {
			return fmt.Errorf("missing criterion verdicts in %s", key)
		}
	}
	if len(seen) != 2*len(scenarios) {
		return fmt.Errorf("incomplete paired runs: got %d, want %d", len(seen), 2*len(scenarios))
	}
	return nil
}

func containsAuthoringID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func checkAuthoringAnchor(dir, anchor string) error {
	name, line, hasLine := strings.Cut(anchor, "#L")
	if !filepath.IsLocal(name) {
		return fmt.Errorf("anchor must be report-relative: %q", anchor)
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("missing anchor %q: %w", anchor, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("anchor escapes report directory: %q", anchor)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("empty/unreadable anchor %q", anchor)
	}
	if hasLine {
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(strings.Split(strings.TrimRight(string(data), "\n"), "\n")) {
			return fmt.Errorf("invalid line anchor %q", anchor)
		}
	}
	return nil
}

func syntheticAuthoringReport(scenarios []authoringScenario, hashes map[string]string) authoringReport {
	report := authoringReport{Kind: "synthetic", BaselineCommit: "3995cab96c79f2de891f7c86ed559d48d3a7253a", Agent: "fixture", AgentVersion: "fixture", Model: "fixture", Settings: "fixture", Baseline: hashes, Candidate: hashes}
	for _, variant := range []string{"baseline", "candidate"} {
		for _, scenario := range scenarios {
			run := authoringRun{Variant: variant, Scenario: scenario.ID, Verdict: "pass", Transcript: "transcript.txt", Artifact: "artifact.txt"}
			for _, id := range scenario.Criteria {
				run.Checks = append(run.Checks, authoringCheck{ID: id, Verdict: "pass", Anchor: "transcript.txt#L1"})
			}
			report.Runs = append(report.Runs, run)
		}
	}
	return report
}

func TestAuthoringEvalReportContract(t *testing.T) {
	scenarios := readAuthoringScenarios(t)
	hashes := bundleHashes(t)
	cases := []struct {
		name      string
		mutate    func(*authoringReport)
		wantError bool
	}{
		{"complete synthetic contract", func(*authoringReport) {}, false},
		{"baseline failure is comparison data", func(r *authoringReport) { r.Runs[0].Verdict = "fail"; r.Runs[0].Checks[0].Verdict = "fail" }, false},
		{"missing run", func(r *authoringReport) { r.Runs = r.Runs[1:] }, true},
		{"duplicate run", func(r *authoringReport) { r.Runs[1] = r.Runs[0] }, true},
		{"stale candidate", func(r *authoringReport) { r.Candidate[authoringBundleFiles[0]] = "stale" }, true},
		{"missing provenance", func(r *authoringReport) { r.Model = "" }, true},
		{"not run", func(r *authoringReport) { r.Runs[5].Verdict = "not-run" }, true},
		{"failed candidate", func(r *authoringReport) { r.Runs[5].Verdict = "fail" }, true},
		{"missing criterion", func(r *authoringReport) { r.Runs[5].Checks = nil }, true},
		{"failed criterion", func(r *authoringReport) { r.Runs[5].Checks[0].Verdict = "fail" }, true},
		{"missing anchor", func(r *authoringReport) { r.Runs[5].Transcript = "missing.txt" }, true},
		{"escaped anchor", func(r *authoringReport) { r.Runs[5].Transcript = "../outside.txt" }, true},
		{"bad line", func(r *authoringReport) { r.Runs[5].Checks[0].Anchor = "transcript.txt#L999" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, file := range []string{"transcript.txt", "artifact.txt"} {
				if err := os.WriteFile(filepath.Join(dir, file), []byte("Synthetic test data, not an observed run.\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// Round-trip prevents the negative cases from sharing nested maps.
			data, _ := json.Marshal(syntheticAuthoringReport(scenarios, hashes))
			var report authoringReport
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&report)
			err := checkAuthoringReport(report, scenarios, hashes, dir, false)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %t", err, tc.wantError)
			}
			if !tc.wantError && checkAuthoringReport(report, scenarios, hashes, dir, true) == nil {
				t.Fatal("synthetic fixture was accepted as observed behavior")
			}
		})
	}
}

func TestAuthoringBehavioralAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_AUTHORING_EVAL_REPORT")
	if path == "" {
		t.Skip("behavioral acceptance not run: set WALDEN_AUTHORING_EVAL_REPORT to a reviewed observed report")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report authoringReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	if err := checkAuthoringReport(report, readAuthoringScenarios(t), bundleHashes(t), filepath.Dir(path), true); err != nil {
		t.Fatal(err)
	}
}
