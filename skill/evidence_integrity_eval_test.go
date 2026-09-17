package skill

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

const integrityLabel = "v0.10.2"
const integrityGuideMarker = "\n\n# Exact Walden guide for this session\n\n"
const integrityVersionFlag = "-X github.com/andrearaponi/walden/internal/app.Version=" + integrityLabel

var integrityScenarioIDs = []string{"applicability", "conflicting-intent", "case-study", "authorized-retirement", "retirement-guards", "batch-checkpoints"}

type integrityScenario struct {
	ID        string   `json:"id"`
	Seed      string   `json:"seed"`
	Prompt    string   `json:"prompt"`
	FollowUps []string `json:"follow_ups"`
	Criteria  []string `json:"criteria"`
	Expect    []string `json:"expect"`
}

type integrityEvalRun struct {
	Scenario           string           `json:"scenario"`
	Variant            string           `json:"variant"`
	Verdict            string           `json:"verdict"`
	Transcript         string           `json:"transcript"`
	Artifact           string           `json:"artifact"`
	BinarySHA256       string           `json:"binary_sha256"`
	SkillSHA256        string           `json:"skill_sha256"`
	SystemPrompt       string           `json:"system_prompt"`
	SystemPromptSHA256 string           `json:"system_prompt_sha256"`
	Checks             []authoringCheck `json:"checks"`
}

type integrityEvalReport struct {
	Kind          string             `json:"kind"`
	Label         string             `json:"label"`
	SourceHead    string             `json:"source_head"`
	Agent         string             `json:"agent"`
	AgentVersion  string             `json:"agent_version"`
	Model         string             `json:"model"`
	Settings      string             `json:"settings"`
	GoVersion     string             `json:"go_version"`
	BaselineSkill string             `json:"baseline_skill"`
	Binary        string             `json:"binary"`
	BinarySHA256  string             `json:"binary_sha256"`
	Baseline      map[string]string  `json:"baseline"`
	Candidate     map[string]string  `json:"candidate"`
	Fixtures      map[string]string  `json:"fixtures"`
	Runs          []integrityEvalRun `json:"runs"`
}

func integrityFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func integrityFixtures(t *testing.T) ([]integrityScenario, map[string]string) {
	t.Helper()
	root := filepath.Join(authoringRoot(t), "skill", "testdata", "evidence-integrity-eval")
	data, err := os.ReadFile(filepath.Join(root, "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []integrityScenario
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&scenarios); err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, err := integrityFileHash(path)
		if err != nil {
			return err
		}
		hashes[filepath.ToSlash(rel)] = hash
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return scenarios, hashes
}

// This checker validates reviewed observations and their provenance. It is
// not a model runner, transcript judge, or substitute for semantic review.
func checkIntegrityReport(report integrityEvalReport, scenarios []integrityScenario, bundle, fixtures map[string]string, dir string, observed bool) error {
	if observed && report.Kind != "observed" {
		return fmt.Errorf("acceptance requires observed sessions, not synthetic report data")
	}
	if report.Label != integrityLabel || report.SourceHead == "" || report.Agent == "" || report.AgentVersion == "" || report.Model == "" || report.Settings == "" || report.GoVersion == "" {
		return fmt.Errorf("missing build/agent/model provenance")
	}
	if !reflect.DeepEqual(report.Candidate, bundle) || !reflect.DeepEqual(report.Fixtures, fixtures) {
		return fmt.Errorf("candidate bundle or scenario fixtures changed after observation")
	}
	for path, hash := range report.Baseline {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return fmt.Errorf("invalid baseline identity for %s", path)
		}
	}
	if len(report.Baseline) != len(bundle) {
		return fmt.Errorf("incomplete baseline manifest")
	}
	for name, hash := range bundle {
		if name != "skill/walden/SKILL.md" && report.Baseline[name] != hash {
			return fmt.Errorf("paired scaffolds differ: %s", name)
		}
	}
	for _, path := range []string{report.BaselineSkill, report.Binary} {
		if err := checkAuthoringAnchor(dir, path); err != nil {
			return err
		}
	}
	baseline, err := integrityFileHash(filepath.Join(dir, report.BaselineSkill))
	if err != nil || baseline != report.Baseline["skill/walden/SKILL.md"] {
		return fmt.Errorf("frozen baseline bytes do not match the report")
	}
	binary, err := integrityFileHash(filepath.Join(dir, report.Binary))
	if err != nil || binary != report.BinarySHA256 {
		return fmt.Errorf("observed CLI binary changed or is missing")
	}
	catalog := map[string]integrityScenario{}
	for _, scenario := range scenarios {
		catalog[scenario.ID] = scenario
	}
	seen := map[string]bool{}
	for _, run := range report.Runs {
		scenario, exists := catalog[run.Scenario]
		key := run.Variant + ":" + run.Scenario
		if !exists || seen[key] || (run.Variant != "baseline" && run.Variant != "candidate") {
			return fmt.Errorf("unexpected/duplicate run %s", key)
		}
		seen[key] = true
		if run.Verdict != "pass" && run.Verdict != "fail" {
			return fmt.Errorf("run %s was not exercised", key)
		}
		if run.Variant == "candidate" && run.Verdict != "pass" {
			return fmt.Errorf("candidate observation failed: %s", key)
		}
		skillHash := report.Candidate["skill/walden/SKILL.md"]
		if run.Variant == "baseline" {
			skillHash = baseline
		}
		if run.BinarySHA256 != binary || run.SkillSHA256 != skillHash {
			return fmt.Errorf("run %s used a different CLI or guide", key)
		}
		if err := checkAuthoringAnchor(dir, run.SystemPrompt); err != nil {
			return fmt.Errorf("%s system prompt: %w", key, err)
		}
		prompt, err := os.ReadFile(filepath.Join(dir, run.SystemPrompt))
		if err != nil {
			return err
		}
		promptHash := sha256.Sum256(prompt)
		_, guide, found := strings.Cut(string(prompt), integrityGuideMarker)
		guideHash := sha256.Sum256([]byte(guide))
		if !found || hex.EncodeToString(promptHash[:]) != run.SystemPromptSHA256 || hex.EncodeToString(guideHash[:]) != skillHash {
			return fmt.Errorf("run %s did not pin its declared guide in the supplied prompt", key)
		}
		for _, anchor := range []string{run.Transcript, run.Artifact} {
			if err := checkAuthoringAnchor(dir, anchor); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
		checks := map[string]bool{}
		for _, check := range run.Checks {
			if checks[check.ID] || !containsAuthoringID(scenario.Criteria, check.ID) {
				return fmt.Errorf("invalid criterion %s in %s", check.ID, key)
			}
			checks[check.ID] = true
			if (check.Verdict != "pass" && check.Verdict != "fail") || ((run.Variant == "candidate" || run.Verdict == "pass") && check.Verdict != "pass") {
				return fmt.Errorf("criterion %s not satisfied in %s", check.ID, key)
			}
			if err := checkAuthoringAnchor(dir, check.Anchor); err != nil {
				return err
			}
		}
		if len(checks) != len(scenario.Criteria) {
			return fmt.Errorf("missing criteria in %s", key)
		}
	}
	if len(seen) != 2*len(scenarios) {
		return fmt.Errorf("incomplete paired sessions: %d, want %d", len(seen), 2*len(scenarios))
	}
	return nil
}

func TestEvidenceIntegritySkillSupport(t *testing.T) {
	t.Run("guidance", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(authoringRoot(t), "skill/walden/SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		requireAuthoringText(t, string(data), "current contract", "superseded", "mixed", "unresolved", "unattested", "checkpoint", "execution-integrity", "not a repository-wide", "last-live commit")
		if strings.Contains(string(data), "run `walden verify <feature> --all` for each affected completed feature") {
			t.Error("unconditional forced historical replay remains")
		}
	})
	t.Run("documentation", func(t *testing.T) {
		for path, fragments := range map[string][]string{
			"docs/adoption.md":       {"current contract", "unattested", "successor", "read-only"},
			"docs/lifecycle.md":      {"contamination", "unattested", "scope"},
			"docs/reference/json.md": {"v1alpha2", "unattested", "input_binding", "execution", "scope"},
			"docs/reference/cli.md":  {"unattested", "contamination", "probe"},
		} {
			data, err := os.ReadFile(filepath.Join(authoringRoot(t), path))
			if err != nil {
				t.Fatal(err)
			}
			requireAuthoringText(t, string(data), fragments...)
			if strings.Contains(string(data), "One misbehaving proof fails alone") {
				t.Error("unsafe run-start guarantee remains")
			}
		}
	})
	scenarios, fixtures := integrityFixtures(t)
	ids, criteria := []string{}, map[string]bool{}
	for _, scenario := range scenarios {
		ids = append(ids, scenario.ID)
		if scenario.Prompt == "" || scenario.Seed == "" || len(scenario.Expect) == 0 || len(scenario.Criteria) == 0 {
			t.Fatalf("incomplete scenario %s", scenario.ID)
		}
		for _, id := range scenario.Criteria {
			criteria[id] = true
		}
	}
	if !reflect.DeepEqual(ids, integrityScenarioIDs) || len(criteria) != 4 {
		t.Fatalf("scenario/AC catalog incomplete: %v %v", ids, criteria)
	}
	for _, path := range []string{"README.md", "portfolio.json", "seed/product-decisions.md", "seed/constitution.md", "seed/format.go", "seed/format_test.go"} {
		if fixtures[path] == "" {
			t.Fatalf("missing fixture %s", path)
		}
	}
	t.Run("report negative controls", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			mutate func(*integrityEvalReport)
			bad    bool
		}{
			{"synthetic structural control", func(*integrityEvalReport) {}, false},
			{"baseline failure allowed", func(r *integrityEvalReport) { r.Runs[0].Verdict = "fail"; r.Runs[0].Checks[0].Verdict = "fail" }, false},
			{"missing run", func(r *integrityEvalReport) { r.Runs = r.Runs[1:] }, true},
			{"duplicate run", func(r *integrityEvalReport) { r.Runs[1] = r.Runs[0] }, true},
			{"stale candidate", func(r *integrityEvalReport) { r.Candidate["skill/walden/SKILL.md"] = "changed" }, true},
			{"stale fixtures", func(r *integrityEvalReport) { r.Fixtures["scenarios.json"] = "changed" }, true},
			{"missing model", func(r *integrityEvalReport) { r.Model = "" }, true},
			{"different CLI", func(r *integrityEvalReport) { r.Runs[6].BinarySHA256 = "different" }, true},
			{"different guide", func(r *integrityEvalReport) { r.Runs[6].SkillSHA256 = "different" }, true},
			{"guide not supplied", func(r *integrityEvalReport) { r.Runs[6].SystemPrompt = "transcript.txt" }, true},
			{"changed system prompt", func(r *integrityEvalReport) { r.Runs[6].SystemPromptSHA256 = "different" }, true},
			{"unrun candidate", func(r *integrityEvalReport) { r.Runs[6].Verdict = "not-run" }, true},
			{"failed candidate", func(r *integrityEvalReport) { r.Runs[6].Verdict = "fail" }, true},
			{"missing criterion", func(r *integrityEvalReport) { r.Runs[6].Checks = nil }, true},
			{"missing anchor", func(r *integrityEvalReport) { r.Runs[6].Transcript = "missing.txt" }, true},
			{"escaped anchor", func(r *integrityEvalReport) { r.Runs[6].Artifact = "../outside.txt" }, true},
		} {
			t.Run(test.name, func(t *testing.T) {
				dir := t.TempDir()
				for _, name := range []string{"baseline.md", "walden", "transcript.txt", "artifact.txt"} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic fixture, not observed behavior\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				hash, _ := integrityFileHash(filepath.Join(dir, "walden"))
				bundle := bundleHashes(t)
				baseline := map[string]string{}
				for name, value := range bundle {
					baseline[name] = value
				}
				baseline["skill/walden/SKILL.md"] = hash
				report := integrityEvalReport{Kind: "synthetic", Label: integrityLabel, SourceHead: "fixture", Agent: "fixture", AgentVersion: "fixture", Model: "fixture", Settings: "fixture", GoVersion: "fixture", BaselineSkill: "baseline.md", Binary: "walden", BinarySHA256: hash, Baseline: baseline, Candidate: bundle, Fixtures: fixtures}
				for _, variant := range []string{"baseline", "candidate"} {
					for _, scenario := range scenarios {
						skillHash := bundle["skill/walden/SKILL.md"]
						if variant == "baseline" {
							skillHash = hash
						}
						guidePath := filepath.Join(authoringRoot(t), "skill/walden/SKILL.md")
						if variant == "baseline" {
							guidePath = filepath.Join(dir, "baseline.md")
						}
						guide, err := os.ReadFile(guidePath)
						if err != nil {
							t.Fatal(err)
						}
						promptFile := variant + "-system.txt"
						if err := os.WriteFile(filepath.Join(dir, promptFile), []byte("synthetic operations"+integrityGuideMarker+string(guide)), 0o600); err != nil {
							t.Fatal(err)
						}
						promptHash, err := integrityFileHash(filepath.Join(dir, promptFile))
						if err != nil {
							t.Fatal(err)
						}
						run := integrityEvalRun{Variant: variant, Scenario: scenario.ID, Verdict: "pass", Transcript: "transcript.txt", Artifact: "artifact.txt", BinarySHA256: hash, SkillSHA256: skillHash, SystemPrompt: promptFile, SystemPromptSHA256: promptHash}
						for _, id := range scenario.Criteria {
							run.Checks = append(run.Checks, authoringCheck{ID: id, Verdict: "pass", Anchor: "transcript.txt#L1"})
						}
						report.Runs = append(report.Runs, run)
					}
				}
				data, _ := json.Marshal(report)
				var copied integrityEvalReport
				if err := json.Unmarshal(data, &copied); err != nil {
					t.Fatal(err)
				}
				test.mutate(&copied)
				err := checkIntegrityReport(copied, scenarios, bundle, fixtures, dir, false)
				if (err != nil) != test.bad {
					t.Fatalf("error=%v want failure=%t", err, test.bad)
				}
				if !test.bad && checkIntegrityReport(copied, scenarios, bundle, fixtures, dir, true) == nil {
					t.Fatal("synthetic data accepted as observed behavior")
				}
			})
		}
	})
}

func TestEvidenceIntegritySkillObservedAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_EVIDENCE_INTEGRITY_EVAL_REPORT")
	if path == "" {
		t.Skip("observed skill pilot not run: provide WALDEN_EVIDENCE_INTEGRITY_EVAL_REPORT")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report integrityEvalReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	scenarios, fixtures := integrityFixtures(t)
	if err := checkIntegrityReport(report, scenarios, bundleHashes(t), fixtures, filepath.Dir(path), true); err != nil {
		t.Fatal(err)
	}
	if report.GoVersion != runtime.Version() {
		t.Fatalf("observed Go toolchain %s differs from current %s", report.GoVersion, runtime.Version())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "walden")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags", integrityVersionFlag, "-o", binary, "./cmd/walden")
	build.Dir = authoringRoot(t)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=auto")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile current candidate: %v (%s)", err, out)
	}
	hash, err := integrityFileHash(binary)
	if err != nil || hash != report.BinarySHA256 {
		t.Fatalf("observed CLI does not match current production content: %s != %s (%v)", report.BinarySHA256, hash, err)
	}
}
