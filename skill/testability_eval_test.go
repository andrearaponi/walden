package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Behavioral evaluation assets of the testability companion
// (testability-skill, Verification Plan). The fixtures are synthetic inputs
// and a rubric for reviewing observed runs, not a runtime dependency of the
// skill and not an automated judge of model behavior.

const testabilityEvalDir = "testdata/testability-eval"

type testabilitySetup struct {
	Repository   string `json:"repository"`
	Go           string `json:"go,omitempty"`
	Requirements string `json:"requirements,omitempty"`
	Design       string `json:"design,omitempty"`
	DesignStatus string `json:"design_status,omitempty"`
	WaldenOnPath bool   `json:"walden_on_path"`
	Notes        string `json:"notes"`
}

type testabilityScenario struct {
	ID        string              `json:"id"`
	Title     string              `json:"title"`
	Setup     testabilitySetup    `json:"setup"`
	Skills    map[string][]string `json:"skills"`
	Prompt    string              `json:"prompt"`
	FollowUps *[]string           `json:"follow_ups"`
	Criteria  []string            `json:"criteria"`
	Expect    []string            `json:"expect"`
}

// testabilityFeatureIDs lists the identifiers of the testability-skill
// requirements a scenario may observe: its 27 acceptance criteria and its
// quality attributes.
func testabilityFeatureIDs() (criteria []string, all map[string]bool) {
	counts := map[string]int{"R1": 5, "R2": 6, "R3": 2, "R4": 4, "R5": 5, "R6": 3, "R7": 2}
	all = map[string]bool{}
	for requirement, n := range counts {
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("%s.AC%d", requirement, i)
			criteria = append(criteria, id)
			all[id] = true
		}
	}
	for i := 1; i <= 5; i++ {
		all[fmt.Sprintf("NFR%d", i)] = true
	}
	sort.Strings(criteria)
	return criteria, all
}

func parseTestabilityScenarios(data []byte) ([]testabilityScenario, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var scenarios []testabilityScenario
	if err := decoder.Decode(&scenarios); err != nil {
		return nil, err
	}
	return scenarios, nil
}

func readTestabilityScenarios(t *testing.T) []testabilityScenario {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testabilityEvalDir, "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := parseTestabilityScenarios(data)
	if err != nil {
		t.Fatalf("scenarios.json: %v", err)
	}
	return scenarios
}

func sameTestabilitySkills(got, want []string) bool {
	return strings.Join(got, "\n") == strings.Join(want, "\n")
}

// checkTestabilityScenarios validates the scenario catalog: S1 to S6, A1 and
// A2, each complete, with the skills the Verification Plan assigns, and
// together listing every acceptance criterion of the feature.
func checkTestabilityScenarios(scenarios []testabilityScenario) error {
	criteria, known := testabilityFeatureIDs()
	want := map[string]bool{"S1": true, "S2": true, "S3": true, "S4": true, "S5": true, "S6": true, "A1": true, "A2": true}
	listed := map[string]bool{}
	pair := []string{"skill/walden", "skill/walden-testability"}
	for _, scenario := range scenarios {
		if !want[scenario.ID] {
			return fmt.Errorf("unexpected or duplicate scenario %q", scenario.ID)
		}
		delete(want, scenario.ID)
		switch {
		case scenario.Title == "" || scenario.Setup.Repository == "" || scenario.Setup.Notes == "":
			return fmt.Errorf("%s: incomplete setup", scenario.ID)
		case scenario.Prompt == "":
			return fmt.Errorf("%s: no prompt", scenario.ID)
		case scenario.FollowUps == nil:
			return fmt.Errorf("%s: no follow_ups list", scenario.ID)
		case len(scenario.Criteria) == 0 || len(scenario.Expect) == 0:
			return fmt.Errorf("%s: no criteria or expectations", scenario.ID)
		case len(scenario.Skills["candidate"]) == 0:
			return fmt.Errorf("%s: no candidate skills", scenario.ID)
		}
		for variant := range scenario.Skills {
			if variant != "candidate" && variant != "baseline" {
				return fmt.Errorf("%s: unknown variant %q", scenario.ID, variant)
			}
		}
		candidate, baseline := scenario.Skills["candidate"], scenario.Skills["baseline"]
		if scenario.ID == "S6" {
			if !sameTestabilitySkills(candidate, []string{"skill/walden-testability"}) {
				return fmt.Errorf("S6 must load the candidate alone, got %v", candidate)
			}
		} else if !sameTestabilitySkills(candidate, pair) {
			return fmt.Errorf("%s: the candidate loads %v, want %v", scenario.ID, candidate, pair)
		}
		hasBaseline := scenario.ID == "S1" || scenario.ID == "S2"
		if hasBaseline != (baseline != nil) || (hasBaseline && !sameTestabilitySkills(baseline, []string{"skill/walden"})) {
			return fmt.Errorf("%s: the baseline loads %v; only S1 and S2 have one, with the walden guide alone", scenario.ID, baseline)
		}
		if (scenario.ID == "S5") == scenario.Setup.WaldenOnPath {
			return fmt.Errorf("%s: walden_on_path is %t", scenario.ID, scenario.Setup.WaldenOnPath)
		}
		for _, id := range scenario.Criteria {
			if !known[id] {
				return fmt.Errorf("%s lists %q, which is not a criterion of testability-skill", scenario.ID, id)
			}
			listed[id] = true
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("missing scenarios %v", want)
	}
	for _, id := range criteria {
		if !listed[id] {
			return fmt.Errorf("no scenario lists %s", id)
		}
	}
	for _, scenario := range scenarios {
		switch {
		case scenario.ID == "S1" && strings.Contains(strings.ToLower(scenario.Prompt), "walden-testability"):
			return fmt.Errorf("S1 names the companion in its prompt")
		case scenario.ID == "S2" && len(*scenario.FollowUps) == 0:
			return fmt.Errorf("S2 has no follow-up accepting and rejecting changes")
		}
	}
	return nil
}

// The planted cases and negative controls of the Verification Plan, each
// needing one row in the README's expected findings.
var testabilityPlantedCases = []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "N1", "N2", "N3"}

// checkTestabilityFindings requires one README row per planted case, with a
// criterion, an existing location under the fixture directory and a
// non-empty expected finding.
func checkTestabilityFindings(readme string, exists func(rel string) bool) error {
	rows := map[string][]string{}
	for _, line := range strings.Split(readme, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		var cells []string
		for _, cell := range strings.Split(strings.Trim(line, "|"), "|") {
			cells = append(cells, strings.TrimSpace(cell))
		}
		if len(cells) >= 5 {
			if _, duplicate := rows[cells[0]]; duplicate {
				return fmt.Errorf("planted case %s has two rows", cells[0])
			}
			rows[cells[0]] = cells
		}
	}
	for _, id := range testabilityPlantedCases {
		cells, found := rows[id]
		if !found {
			return fmt.Errorf("no expected finding for planted case %s", id)
		}
		location := strings.Trim(cells[2], "`")
		if cells[1] == "" || cells[3] == "" || !exists(location) {
			return fmt.Errorf("planted case %s: criterion %q, location %q, finding %q", id, cells[1], cells[2], cells[3])
		}
	}
	return nil
}

// checkNoWaldenDirectory rejects a fixture path holding a directory named
// .walden, which this repository ignores.
func checkNoWaldenDirectory(paths []string) error {
	for _, path := range paths {
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			if part == ".walden" {
				return fmt.Errorf("fixture path %q holds a .walden directory", path)
			}
		}
	}
	return nil
}

func TestTestabilityEvalFixtures(t *testing.T) {
	scenarios := readTestabilityScenarios(t)

	t.Run("scenarios", func(t *testing.T) {
		if _, err := parseTestabilityScenarios([]byte(`[{"id":"S1","surprise":true}]`)); err == nil {
			t.Error("a scenario with an unknown field was accepted")
		}
		clone := func() []testabilityScenario {
			data, _ := json.Marshal(scenarios)
			copied, err := parseTestabilityScenarios(data)
			if err != nil {
				t.Fatal(err)
			}
			return copied
		}
		if err := checkTestabilityScenarios(clone()); err != nil {
			t.Fatalf("scenarios.json: %v", err)
		}
		for name, mutate := range map[string]func([]testabilityScenario) []testabilityScenario{
			"a missing scenario":   func(s []testabilityScenario) []testabilityScenario { return s[1:] },
			"a duplicate scenario": func(s []testabilityScenario) []testabilityScenario { s[1] = s[0]; return s },
			"an empty prompt":      func(s []testabilityScenario) []testabilityScenario { s[2].Prompt = ""; return s },
			"no follow-up list":    func(s []testabilityScenario) []testabilityScenario { s[3].FollowUps = nil; return s },
			"no expectations":      func(s []testabilityScenario) []testabilityScenario { s[4].Expect = nil; return s },
			"no setup notes":       func(s []testabilityScenario) []testabilityScenario { s[5].Setup.Notes = ""; return s },
			"a foreign criterion": func(s []testabilityScenario) []testabilityScenario {
				s[0].Criteria = append(s[0].Criteria, "R9.AC1")
				return s
			},
			"an unlisted criterion": func(s []testabilityScenario) []testabilityScenario { return dropTestabilityCriterion(s, "R6.AC2") },
			"a baseline beyond S1 and S2": func(s []testabilityScenario) []testabilityScenario {
				return withTestabilitySkills(s, "S3", "baseline", []string{"skill/walden"})
			},
			"a companion loaded with the guide in S6": func(s []testabilityScenario) []testabilityScenario {
				return withTestabilitySkills(s, "S6", "candidate", []string{"skill/walden", "skill/walden-testability"})
			},
		} {
			if checkTestabilityScenarios(mutate(clone())) == nil {
				t.Errorf("a catalog with %s was accepted", name)
			}
		}
	})

	t.Run("expected findings", func(t *testing.T) {
		exists := func(rel string) bool {
			if rel == "requirements" {
				rel = "seed/requirements.md"
			}
			info, err := os.Stat(filepath.Join(testabilityEvalDir, filepath.FromSlash(rel)))
			return err == nil && !info.IsDir()
		}
		const row = "| P1 | R2.AC1 | `seed/requirements.md` | Clock gap. | Any Go seam. |\n"
		fixtureExists := func(rel string) bool { return rel == "seed/requirements.md" }
		if checkTestabilityFindings(row, fixtureExists) == nil {
			t.Error("a README covering one planted case was accepted")
		}
		var complete strings.Builder
		for _, id := range testabilityPlantedCases {
			complete.WriteString(strings.Replace(row, "P1", id, 1))
		}
		if err := checkTestabilityFindings(complete.String(), fixtureExists); err != nil {
			t.Fatalf("a complete fixture was rejected: %v", err)
		}
		if checkTestabilityFindings(strings.Replace(complete.String(), "| P4 | R2.AC1 | `seed/requirements.md` | Clock gap. |", "| P4 | R2.AC1 | `seed/requirements.md` |  |", 1), fixtureExists) == nil {
			t.Error("a planted case without an expected finding was accepted")
		}
		if checkTestabilityFindings(strings.Replace(complete.String(), "| P5 | R2.AC1 | `seed/requirements.md`", "| P5 | R2.AC1 | `seed/missing.go`", 1), fixtureExists) == nil {
			t.Error("a planted case at a missing location was accepted")
		}
		readme, err := os.ReadFile(filepath.Join(testabilityEvalDir, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := checkTestabilityFindings(string(readme), exists); err != nil {
			t.Errorf("README.md: %v", err)
		}
	})

	t.Run("no .walden directory", func(t *testing.T) {
		if checkNoWaldenDirectory([]string{"seed/.walden/constitution.md"}) == nil {
			t.Error("a fixture under .walden was accepted")
		}
		var paths []string
		err := filepath.WalkDir(testabilityEvalDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) < 10 {
			t.Fatalf("the fixture tree holds only %d paths", len(paths))
		}
		if err := checkNoWaldenDirectory(paths); err != nil {
			t.Error(err)
		}
	})
}

func dropTestabilityCriterion(scenarios []testabilityScenario, id string) []testabilityScenario {
	for i := range scenarios {
		var kept []string
		for _, criterion := range scenarios[i].Criteria {
			if criterion != id {
				kept = append(kept, criterion)
			}
		}
		scenarios[i].Criteria = kept
	}
	return scenarios
}

func withTestabilitySkills(scenarios []testabilityScenario, id, variant string, skills []string) []testabilityScenario {
	for i := range scenarios {
		if scenarios[i].ID == id {
			scenarios[i].Skills[variant] = skills
		}
	}
	return scenarios
}

// The candidate files whose content a report must have run.
var testabilityCandidateFiles = []string{testabilitySkillPath, testabilitySeamsPath}

type testabilityCheck struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Anchor  string `json:"anchor"`
	Note    string `json:"note,omitempty"`
}

type testabilityRun struct {
	Scenario   string             `json:"scenario"`
	Variant    string             `json:"variant"`
	Verdict    string             `json:"verdict"`
	Transcript string             `json:"transcript"`
	Feature    string             `json:"feature,omitempty"`
	Notes      string             `json:"notes,omitempty"`
	Checks     []testabilityCheck `json:"checks"`
}

type testabilityReport struct {
	Kind          string            `json:"kind"`
	Host          string            `json:"host"`
	HostVersion   string            `json:"host_version"`
	Model         string            `json:"model"`
	Settings      string            `json:"settings"`
	WaldenVersion string            `json:"walden_version"`
	Candidate     map[string]string `json:"candidate"`
	Runs          []testabilityRun  `json:"runs"`
}

func testabilityCandidateHashes(t *testing.T) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	for _, rel := range testabilityCandidateFiles {
		data, err := os.ReadFile(filepath.Join(authoringRoot(t), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
	}
	return hashes
}

// checkTestabilityReport validates the integrity of an evaluation report. It
// is a test-only check over reviewed observations, not an agent runner and
// not a judge of the transcripts.
func checkTestabilityReport(report testabilityReport, scenarios []testabilityScenario, current map[string]string, dir string, observed bool) error {
	if observed && report.Kind != "observed" {
		return fmt.Errorf("acceptance requires observed runs, not kind %q", report.Kind)
	}
	if report.Host == "" || report.HostVersion == "" || report.Model == "" || report.WaldenVersion == "" {
		return fmt.Errorf("missing host, host version, model or walden version")
	}
	for name, hash := range current {
		if report.Candidate[name] != hash {
			return fmt.Errorf("the report did not run the current %s", name)
		}
	}
	if err := checkTestabilityScenarios(scenarios); err != nil {
		return fmt.Errorf("scenario catalog: %w", err)
	}
	catalog := map[string]testabilityScenario{}
	want := map[string]bool{}
	for _, scenario := range scenarios {
		catalog[scenario.ID] = scenario
		for variant := range scenario.Skills {
			want[variant+":"+scenario.ID] = true
		}
	}
	for _, run := range report.Runs {
		key := run.Variant + ":" + run.Scenario
		if !want[key] {
			return fmt.Errorf("unexpected or duplicate run %s", key)
		}
		delete(want, key)
		scenario := catalog[run.Scenario]
		if run.Verdict != "pass" && run.Verdict != "fail" {
			return fmt.Errorf("run %s was not exercised", key)
		}
		if run.Variant == "candidate" && run.Verdict != "pass" {
			return fmt.Errorf("candidate run %s did not pass", key)
		}
		if scenario.Setup.Repository != "seed" && run.Feature == "" {
			return fmt.Errorf("run %s does not name the adopter feature it reviewed", key)
		}
		if err := checkTestabilityAnchor(dir, run.Transcript); err != nil {
			return fmt.Errorf("%s transcript: %w", key, err)
		}
		checked := map[string]bool{}
		for _, check := range run.Checks {
			if checked[check.ID] || !containsAuthoringID(scenario.Criteria, check.ID) {
				return fmt.Errorf("unexpected or duplicate criterion %s in %s", check.ID, key)
			}
			checked[check.ID] = true
			switch {
			case check.Verdict != "pass" && check.Verdict != "fail" && check.Verdict != "not-run":
				return fmt.Errorf("criterion %s in %s has verdict %q", check.ID, key, check.Verdict)
			case run.Variant == "candidate" && check.Verdict != "pass":
				return fmt.Errorf("candidate criterion %s in %s is %s", check.ID, key, check.Verdict)
			case run.Verdict == "pass" && check.Verdict != "pass":
				return fmt.Errorf("run %s passed with criterion %s %s", key, check.ID, check.Verdict)
			}
			if err := checkTestabilityAnchor(dir, check.Anchor); err != nil {
				return fmt.Errorf("%s/%s: %w", key, check.ID, err)
			}
		}
		if len(checked) != len(scenario.Criteria) {
			return fmt.Errorf("run %s judges %d of its %d criteria", key, len(checked), len(scenario.Criteria))
		}
	}
	if len(want) != 0 {
		var missing []string
		for key := range want {
			missing = append(missing, key)
		}
		sort.Strings(missing)
		return fmt.Errorf("missing runs %v", missing)
	}
	return nil
}

// checkTestabilityAnchor requires a report-relative path to an existing,
// non-empty file inside the report directory, with an optional #L<n> line.
func checkTestabilityAnchor(dir, anchor string) error {
	if anchor == "" {
		return fmt.Errorf("missing anchor")
	}
	return checkAuthoringAnchor(dir, anchor)
}

func syntheticTestabilityReport(scenarios []testabilityScenario, hashes map[string]string) testabilityReport {
	report := testabilityReport{Kind: "synthetic", Host: "fixture", HostVersion: "fixture", Model: "fixture", Settings: "fixture", WaldenVersion: "fixture", Candidate: map[string]string{}}
	for name, hash := range hashes {
		report.Candidate[name] = hash
	}
	for _, scenario := range scenarios {
		variants := []string{"candidate"}
		if scenario.Skills["baseline"] != nil {
			variants = append(variants, "baseline")
		}
		for _, variant := range variants {
			run := testabilityRun{Scenario: scenario.ID, Variant: variant, Verdict: "pass", Transcript: "transcript.txt"}
			if scenario.Setup.Repository != "seed" {
				run.Feature = "fixture-feature"
			}
			for _, id := range scenario.Criteria {
				run.Checks = append(run.Checks, testabilityCheck{ID: id, Verdict: "pass", Anchor: "transcript.txt#L1"})
			}
			report.Runs = append(report.Runs, run)
		}
	}
	return report
}

// testabilityRunIndex finds a run of the synthetic report.
func testabilityRunIndex(r *testabilityReport, scenario, variant string) int {
	for i, run := range r.Runs {
		if run.Scenario == scenario && run.Variant == variant {
			return i
		}
	}
	panic("no run " + variant + ":" + scenario)
}

func TestTestabilityEvalReportContract(t *testing.T) {
	scenarios := readTestabilityScenarios(t)
	hashes := testabilityCandidateHashes(t)
	run := func(r *testabilityReport, scenario, variant string) *testabilityRun {
		return &r.Runs[testabilityRunIndex(r, scenario, variant)]
	}
	cases := []struct {
		name      string
		mutate    func(*testabilityReport)
		wantError bool
	}{
		{"complete synthetic contract", func(*testabilityReport) {}, false},
		{"baseline failure is comparison data", func(r *testabilityReport) {
			b := run(r, "S1", "baseline")
			b.Verdict, b.Checks[0].Verdict = "fail", "fail"
		}, false},
		{"missing host", func(r *testabilityReport) { r.Host = "" }, true},
		{"missing host version", func(r *testabilityReport) { r.HostVersion = "" }, true},
		{"missing model", func(r *testabilityReport) { r.Model = "" }, true},
		{"missing walden version", func(r *testabilityReport) { r.WaldenVersion = "" }, true},
		{"missing scenario", func(r *testabilityReport) {
			i := testabilityRunIndex(r, "A2", "candidate")
			r.Runs = append(r.Runs[:i], r.Runs[i+1:]...)
		}, true},
		{"missing baseline on S1", func(r *testabilityReport) {
			i := testabilityRunIndex(r, "S1", "baseline")
			r.Runs = append(r.Runs[:i], r.Runs[i+1:]...)
		}, true},
		{"missing baseline on S2", func(r *testabilityReport) {
			i := testabilityRunIndex(r, "S2", "baseline")
			r.Runs = append(r.Runs[:i], r.Runs[i+1:]...)
		}, true},
		{"baseline on a scenario without one", func(r *testabilityReport) {
			extra := *run(r, "S3", "candidate")
			extra.Variant = "baseline"
			r.Runs = append(r.Runs, extra)
		}, true},
		{"duplicate run", func(r *testabilityReport) { r.Runs = append(r.Runs, *run(r, "S4", "candidate")) }, true},
		{"unknown variant", func(r *testabilityReport) { run(r, "S4", "candidate").Variant = "rerun" }, true},
		{"missing anchor", func(r *testabilityReport) { run(r, "S2", "candidate").Checks[0].Anchor = "" }, true},
		{"absent anchor", func(r *testabilityReport) { run(r, "S2", "candidate").Checks[0].Anchor = "missing.txt" }, true},
		{"escaping anchor", func(r *testabilityReport) { run(r, "S2", "candidate").Checks[0].Anchor = "../outside.txt" }, true},
		{"absolute anchor", func(r *testabilityReport) { run(r, "S2", "candidate").Checks[0].Anchor = "/etc/hostname" }, true},
		{"line beyond the file", func(r *testabilityReport) { run(r, "S2", "candidate").Checks[0].Anchor = "transcript.txt#L999" }, true},
		{"missing transcript", func(r *testabilityReport) { run(r, "S3", "candidate").Transcript = "missing.txt" }, true},
		{"stale SKILL.md hash", func(r *testabilityReport) { r.Candidate[testabilitySkillPath] = "stale" }, true},
		{"stale seams.md hash", func(r *testabilityReport) { r.Candidate[testabilitySeamsPath] = "stale" }, true},
		{"missing candidate hash", func(r *testabilityReport) { delete(r.Candidate, testabilitySeamsPath) }, true},
		{"failed candidate check", func(r *testabilityReport) { run(r, "S1", "candidate").Checks[2].Verdict = "fail" }, true},
		{"candidate check not run", func(r *testabilityReport) { run(r, "S1", "candidate").Checks[2].Verdict = "not-run" }, true},
		{"failed candidate run", func(r *testabilityReport) { run(r, "S5", "candidate").Verdict = "fail" }, true},
		{"candidate run not run", func(r *testabilityReport) { run(r, "S5", "candidate").Verdict = "not-run" }, true},
		{"baseline run not run", func(r *testabilityReport) { run(r, "S2", "baseline").Verdict = "not-run" }, true},
		{"passing run with a failed check", func(r *testabilityReport) { run(r, "S2", "baseline").Checks[0].Verdict = "fail" }, true},
		{"missing check", func(r *testabilityReport) {
			c := run(r, "S6", "candidate")
			c.Checks = c.Checks[:0]
		}, true},
		{"duplicate check", func(r *testabilityReport) {
			c := run(r, "S1", "candidate")
			c.Checks[1] = c.Checks[0]
		}, true},
		{"criterion the scenario does not list", func(r *testabilityReport) {
			c := run(r, "S4", "candidate")
			c.Checks = append(c.Checks, testabilityCheck{ID: "R5.AC5", Verdict: "pass", Anchor: "transcript.txt"})
		}, true},
		{"adopter run without its feature", func(r *testabilityReport) { run(r, "A1", "candidate").Feature = "" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte("Synthetic test data, not an observed run.\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.txt"), []byte("outside\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			// A JSON round trip keeps the cases from sharing slices and maps.
			data, _ := json.Marshal(syntheticTestabilityReport(scenarios, hashes))
			var report testabilityReport
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&report)
			err := checkTestabilityReport(report, scenarios, hashes, dir, false)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %t", err, tc.wantError)
			}
			if !tc.wantError && checkTestabilityReport(report, scenarios, hashes, dir, true) == nil {
				t.Fatal("a synthetic report was accepted as observed behavior")
			}
		})
	}
	t.Run("criterion no scenario lists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte("Synthetic test data, not an observed run.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(scenarios)
		narrowed, err := parseTestabilityScenarios(data)
		if err != nil {
			t.Fatal(err)
		}
		narrowed = dropTestabilityCriterion(narrowed, "R6.AC2")
		if checkTestabilityReport(syntheticTestabilityReport(narrowed, hashes), narrowed, hashes, dir, false) == nil {
			t.Error("a report over a catalog that lists no scenario for R6.AC2 was accepted")
		}
	})
}

func TestTestabilityBehavioralAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_TESTABILITY_EVAL_REPORT")
	if path == "" {
		t.Skip("behavioral acceptance not run: set WALDEN_TESTABILITY_EVAL_REPORT to a reviewed observed report")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report testabilityReport
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	if err := checkTestabilityReport(report, readTestabilityScenarios(t), testabilityCandidateHashes(t), filepath.Dir(path), true); err != nil {
		t.Fatal(err)
	}
}
