package skill

import (
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
