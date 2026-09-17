package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var bootstrapInputs = []string{
	"install.sh", "skill/walden/SKILL.md", "skill/testdata/bootstrap-eval/scenarios.json", "skill/testdata/bootstrap-eval/README.md",
	"README.md", "docs/quickstart.md", "docs/reference/cli.md", "CHANGELOG.md", "RELEASE_NOTES.md",
	"skill/walden/install-claude.md", "skill/walden/install-codex.md", "skill/walden/install-copilot.md", "skill/walden/install-opencode.md",
}

type bootstrapScenario struct {
	ID          string   `json:"id"`
	PathVersion string   `json:"path_version"`
	HomeVersion string   `json:"home_version"`
	Outcome     string   `json:"outcome"`
	Owner       string   `json:"owner"`
	Prompt      string   `json:"prompt"`
	FollowUps   []string `json:"follow_ups"`
	Checks      []string `json:"checks"`
}

type bootstrapRun struct {
	Scenario   string           `json:"scenario"`
	SessionID  string           `json:"session_id"`
	Verdict    string           `json:"verdict"`
	Transcript string           `json:"transcript"`
	Snapshot   string           `json:"snapshot"`
	Turns      int              `json:"turns"`
	Seconds    float64          `json:"seconds"`
	Cost       float64          `json:"cost_usd"`
	Checks     []authoringCheck `json:"checks"`
}

type bootstrapReport struct {
	Kind         string            `json:"kind"`
	Agent        string            `json:"agent"`
	AgentVersion string            `json:"agent_version"`
	Model        string            `json:"model"`
	Settings     string            `json:"settings"`
	Candidate    map[string]string `json:"candidate"`
	Binary       string            `json:"binary"`
	BinarySHA256 string            `json:"binary_sha256"`
	Artifacts    map[string]string `json:"artifacts"`
	Runs         []bootstrapRun    `json:"runs"`
}

func bootstrapReadJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatal("expected exactly one JSON value")
	}
}

func bootstrapHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func bootstrapCurrent(t *testing.T) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, name := range bootstrapInputs {
		result[name] = bootstrapHash(t, filepath.Join(authoringRoot(t), name))
	}
	return result
}

func bootstrapScenarios(t *testing.T) []bootstrapScenario {
	t.Helper()
	var result []bootstrapScenario
	bootstrapReadJSON(t, "testdata/bootstrap-eval/scenarios.json", &result)
	return result
}

func TestBootstrapEvalFixtures(t *testing.T) {
	want := map[string]bool{"compatible-path": true, "compatible-home": true, "missing-denied": true, "upgrade-approved": true, "install-failed": true, "postcheck-failed": true, "manager-overlap": true}
	turns := 0
	for _, scenario := range bootstrapScenarios(t) {
		if !want[scenario.ID] {
			t.Fatalf("unknown/duplicate scenario %q", scenario.ID)
		}
		delete(want, scenario.ID)
		if scenario.Prompt == "" || len(scenario.Checks) == 0 || len(scenario.FollowUps) > 1 {
			t.Fatalf("incomplete scenario %+v", scenario)
		}
		seen := map[string]bool{}
		for _, check := range scenario.Checks {
			if check == "" || seen[check] {
				t.Fatal("invalid criterion inventory")
			}
			seen[check] = true
		}
		turns += 1 + len(scenario.FollowUps)
	}
	if len(want) != 0 || turns != 11 {
		t.Fatalf("missing scenarios %v or wrong turn cap %d", want, turns)
	}
	data, err := os.ReadFile("testdata/bootstrap-eval/README.md")
	if err != nil || len(data) == 0 {
		t.Fatal("missing runbook")
	}
}

// Test-only validation of reviewed records, not a model runner or independent
// natural-language judge. Synthetic fixtures must never pass real acceptance.
func checkBootstrapReport(report bootstrapReport, scenarios []bootstrapScenario, current map[string]string, dir string, observed bool) error {
	if observed && report.Kind != "observed" {
		return fmt.Errorf("real acceptance requires observed sessions")
	}
	if report.Agent == "" || report.AgentVersion == "" || report.Model == "" || report.Settings == "" {
		return fmt.Errorf("missing host/model provenance")
	}
	if !reflect.DeepEqual(report.Candidate, current) {
		return fmt.Errorf("candidate guide, installer, documentation or fixtures changed after observation")
	}
	if err := checkAuthoringAnchor(dir, report.Binary); err != nil {
		return err
	}
	binary, err := os.ReadFile(filepath.Join(dir, report.Binary))
	if err != nil {
		return err
	}
	binaryHash := sha256.Sum256(binary)
	if hex.EncodeToString(binaryHash[:]) != report.BinarySHA256 {
		return fmt.Errorf("candidate binary identity differs")
	}
	checkAnchor := func(anchor string) error {
		if err := checkAuthoringAnchor(dir, anchor); err != nil {
			return err
		}
		name, _, _ := strings.Cut(anchor, "#L")
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if report.Artifacts[name] != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("missing/stale artifact hash for %s", name)
		}
		return nil
	}
	catalog := map[string]bootstrapScenario{}
	for _, scenario := range scenarios {
		catalog[scenario.ID] = scenario
	}
	seen, sessions := map[string]bool{}, map[string]bool{}
	totalCost, turns := 0.0, 0
	for _, run := range report.Runs {
		scenario, ok := catalog[run.Scenario]
		if !ok || seen[run.Scenario] || run.SessionID == "" || sessions[run.SessionID] {
			return fmt.Errorf("missing/duplicate run identity %s", run.Scenario)
		}
		seen[run.Scenario], sessions[run.SessionID] = true, true
		if run.Verdict != "pass" {
			return fmt.Errorf("required case %s did not pass", run.Scenario)
		}
		if run.Turns < 1 || run.Turns > 1+len(scenario.FollowUps) || run.Seconds <= 0 || run.Seconds > 180 || math.IsNaN(run.Seconds) || math.IsNaN(run.Cost) || math.IsInf(run.Cost, 0) || run.Cost < 0 {
			return fmt.Errorf("invalid run accounting %s", run.Scenario)
		}
		totalCost += run.Cost
		turns += run.Turns
		for _, anchor := range []string{run.Transcript, run.Snapshot} {
			if err := checkAnchor(anchor); err != nil {
				return err
			}
		}
		checks := map[string]bool{}
		for _, check := range run.Checks {
			if checks[check.ID] || !containsAuthoringID(scenario.Checks, check.ID) || check.Verdict != "pass" {
				return fmt.Errorf("invalid/failed observation %s/%s", run.Scenario, check.ID)
			}
			checks[check.ID] = true
			if err := checkAnchor(check.Anchor); err != nil {
				return err
			}
		}
		if len(checks) != len(scenario.Checks) {
			return fmt.Errorf("missing observations for %s", run.Scenario)
		}
		if observed {
			data, err := os.ReadFile(filepath.Join(dir, run.Transcript))
			if err != nil {
				return err
			}
			results, calls := 0, 0
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				var event struct {
					Type      string          `json:"type"`
					Subtype   string          `json:"subtype"`
					IsError   bool            `json:"is_error"`
					SessionID string          `json:"session_id"`
					Message   json.RawMessage `json:"message"`
				}
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					return fmt.Errorf("invalid observed transcript: %w", err)
				}
				if event.SessionID != "" && event.SessionID != run.SessionID {
					return fmt.Errorf("transcript session mismatch")
				}
				if event.Type == "result" {
					if event.Subtype != "success" || event.IsError {
						return fmt.Errorf("incomplete model turn")
					}
					results++
				}
				if event.Type == "assistant" {
					var message struct {
						Content []struct {
							Type string `json:"type"`
						} `json:"content"`
					}
					if err := json.Unmarshal(event.Message, &message); err != nil {
						return fmt.Errorf("invalid assistant event")
					}
					for _, block := range message.Content {
						if block.Type == "tool_use" {
							calls++
						}
					}
				}
			}
			if results != run.Turns || calls == 0 {
				return fmt.Errorf("missing actual tool/turn evidence for %s", run.Scenario)
			}
		}
	}
	if len(seen) != len(catalog) || turns > 11 || totalCost > 5 {
		return fmt.Errorf("missing cases or exceeded aggregate allowance")
	}
	return nil
}

func syntheticBootstrapReport(t *testing.T, dir string) bootstrapReport {
	t.Helper()
	for _, name := range []string{"transcript.txt", "snapshot.json", "binary"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Synthetic fixture, not an observed session.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := bootstrapReport{Kind: "synthetic", Agent: "fixture", AgentVersion: "fixture", Model: "fixture", Settings: "fixture", Candidate: bootstrapCurrent(t), Binary: "binary", BinarySHA256: bootstrapHash(t, filepath.Join(dir, "binary")), Artifacts: map[string]string{}}
	for _, name := range []string{"transcript.txt", "snapshot.json"} {
		r.Artifacts[name] = bootstrapHash(t, filepath.Join(dir, name))
	}
	for _, scenario := range bootstrapScenarios(t) {
		run := bootstrapRun{Scenario: scenario.ID, SessionID: scenario.ID, Verdict: "pass", Transcript: "transcript.txt", Snapshot: "snapshot.json", Turns: 1, Seconds: 1, Cost: .01}
		for _, id := range scenario.Checks {
			run.Checks = append(run.Checks, authoringCheck{ID: id, Verdict: "pass", Anchor: "transcript.txt#L1"})
		}
		r.Runs = append(r.Runs, run)
	}
	return r
}

func TestBootstrapEvalReportContract(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*bootstrapReport)
		bad    bool
	}{
		{"complete synthetic structure", func(*bootstrapReport) {}, false},
		{"missing run", func(r *bootstrapReport) { r.Runs = r.Runs[1:] }, true},
		{"duplicate run", func(r *bootstrapReport) { r.Runs[1] = r.Runs[0] }, true},
		{"unknown scenario", func(r *bootstrapReport) { r.Runs[0].Scenario = "unknown" }, true},
		{"missing provenance", func(r *bootstrapReport) { r.AgentVersion = "" }, true},
		{"stale guide", func(r *bootstrapReport) { r.Candidate["skill/walden/SKILL.md"] = "old" }, true},
		{"stale installer", func(r *bootstrapReport) { r.Candidate["install.sh"] = "old" }, true},
		{"stale fixtures", func(r *bootstrapReport) { r.Candidate["skill/testdata/bootstrap-eval/scenarios.json"] = "old" }, true},
		{"stale binary", func(r *bootstrapReport) { r.BinarySHA256 = strings.Repeat("0", 64) }, true},
		{"missing binary", func(r *bootstrapReport) { r.Binary = "missing" }, true},
		{"not run", func(r *bootstrapReport) { r.Runs[0].Verdict = "not-run" }, true},
		{"failed observation", func(r *bootstrapReport) { r.Runs[0].Checks[0].Verdict = "fail" }, true},
		{"missing observation", func(r *bootstrapReport) { r.Runs[0].Checks = nil }, true},
		{"duplicate observation", func(r *bootstrapReport) { r.Runs[0].Checks = append(r.Runs[0].Checks, r.Runs[0].Checks[0]) }, true},
		{"missing anchor", func(r *bootstrapReport) { r.Runs[0].Transcript = "missing.txt" }, true},
		{"escaping anchor", func(r *bootstrapReport) { r.Runs[0].Checks[0].Anchor = "../escape.txt" }, true},
		{"stale artifact", func(r *bootstrapReport) { r.Artifacts["snapshot.json"] = strings.Repeat("0", 64) }, true},
		{"extra turns", func(r *bootstrapReport) { r.Runs[0].Turns = 3 }, true},
		{"invalid cost", func(r *bootstrapReport) { r.Runs[0].Cost = -1 }, true},
		{"over total budget", func(r *bootstrapReport) { r.Runs[0].Cost = 6 }, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			dir := t.TempDir()
			report := syntheticBootstrapReport(t, dir)
			item.mutate(&report)
			err := checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), dir, false)
			if (err != nil) != item.bad {
				t.Fatalf("error=%v, want error=%t", err, item.bad)
			}
			if !item.bad && checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), dir, true) == nil {
				t.Fatal("synthetic report accepted as observed")
			}
		})
	}
	t.Run("synthetic relabeled observed", func(t *testing.T) {
		dir := t.TempDir()
		report := syntheticBootstrapReport(t, dir)
		report.Kind = "observed"
		if checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), dir, true) == nil {
			t.Fatal("non-transcript text accepted as observed tool execution")
		}
	})
	t.Run("client error cannot hide behind success subtype", func(t *testing.T) {
		dir := t.TempDir()
		report := syntheticBootstrapReport(t, dir)
		report.Kind = "observed"
		// A fabricated transcript shape is a checker fixture only. No model
		// behavior is claimed by this positive/negative structure test.
		writeTurn := func(run bootstrapRun, failed bool) string {
			name := run.Scenario + ".jsonl"
			content := fmt.Sprintf("{\"type\":\"assistant\",\"session_id\":%q,\"message\":{\"content\":[{\"type\":\"tool_use\"}]}}\n{\"type\":\"result\",\"session_id\":%q,\"subtype\":\"success\",\"is_error\":%t}\n", run.SessionID, run.SessionID, failed)
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			report.Artifacts[name] = bootstrapHash(t, filepath.Join(dir, name))
			return name
		}
		for i := range report.Runs {
			report.Runs[i].Transcript = writeTurn(report.Runs[i], false)
		}
		if err := checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), dir, true); err != nil {
			t.Fatalf("valid transcript structure rejected: %v", err)
		}
		writeTurn(report.Runs[0], true)
		if checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), dir, true) == nil {
			t.Fatal("client is_error=true accepted as observed success")
		}
	})
}

func TestBootstrapObservedAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_BOOTSTRAP_EVAL_REPORT")
	if path == "" {
		t.Skip("explicit observed bootstrap report not supplied")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	var report bootstrapReport
	bootstrapReadJSON(t, path, &report)
	if err := checkBootstrapReport(report, bootstrapScenarios(t), bootstrapCurrent(t), filepath.Dir(path), true); err != nil {
		t.Fatal(err)
	}
	// A fresh equivalent candidate build is checked by the distribution helper.
	if got := bootstrapHash(t, bootstrapBuild(t)); got != report.BinarySHA256 {
		t.Fatal("observed executable differs from the current candidate")
	}
}

func bootstrapBuild(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "walden")
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", "-X github.com/andrearaponi/walden/internal/app.Version=v0.10.2", "-o", binary, "./cmd/walden")
	cmd.Dir = authoringRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v\n%s", err, output)
	}
	return binary
}
