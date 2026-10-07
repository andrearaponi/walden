package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/ears"
	"github.com/andrearaponi/walden/internal/spec"
)

const (
	releasedGuideSHA256 = "1e9f672cf965c8ac14fa4ebe9eea884d8f7b5af89df4385f3b7665cf91a212e8"
	releasedCLISHA256   = "13da6f3652f33e83e6eb9211e7690b894a1ca780ca87739b4c9c0d14e8d856c2"
)

// approvedConsolidationBudget pins the limits approved in the design.
var approvedConsolidationBudget = consolidationBudget{SessionUSD: 2, SessionSeconds: 900, SessionsUSD: 15, ReviewUSD: 3, TotalUSD: 18}

type consolidationBudget struct {
	SessionUSD     float64 `json:"session_usd"`
	SessionSeconds float64 `json:"session_seconds"`
	SessionsUSD    float64 `json:"sessions_usd"`
	ReviewUSD      float64 `json:"review_usd"`
	TotalUSD       float64 `json:"total_usd"`
}

type consolidationInjection struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Semantic   bool     `json:"semantic"`
	Features   []string `json:"features"`
	Expect     string   `json:"expect"`
	Path       string   `json:"path,omitempty"`
	Identifier string   `json:"identifier,omitempty"`
}

type consolidationChange struct {
	Feature string `json:"feature"`
	Kind    string `json:"kind"`
	Body    string `json:"body"`
}

type consolidationVariant struct {
	ID         string                   `json:"id"`
	Changes    []consolidationChange    `json:"changes"`
	Injections []consolidationInjection `json:"injections"`
}

type discoveryRequest struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Related []string `json:"related"`
	Allowed []string `json:"allowed"`
}

type consolidationScenarios struct {
	Domain               string                 `json:"domain"`
	ConsolidationPrompts map[string]string      `json:"consolidation_prompts"`
	AcceptanceVariants   []string               `json:"acceptance_variants"`
	DevelopmentVariants  []string               `json:"development_variants"`
	Variants             []consolidationVariant `json:"variants"`
	Discovery            []discoveryRequest     `json:"discovery"`
	Schedule             struct {
		Conditions    map[string][]string `json:"conditions"`
		Repetitions   int                 `json:"repetitions"`
		DiscoveryArms []string            `json:"discovery_arms"`
	} `json:"schedule"`
	Thresholds struct {
		ExplicitDetectionMin float64 `json:"explicit_detection_min"`
		ExplicitMarginMin    float64 `json:"explicit_margin_min"`
		PlainDetectionMin    float64 `json:"plain_detection_min"`
		DiscoveryRelatedMin  int     `json:"discovery_related_min"`
	} `json:"thresholds"`
	BaselineWorkflow struct {
		CLISHA256   string `json:"cli_sha256"`
		GuideSHA256 string `json:"guide_sha256"`
	} `json:"baseline_workflow"`
	Budget consolidationBudget `json:"budget"`
}

func consolidationFixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(authoringRoot(t), "skill", "testdata", "consolidation-eval")
}

func decodeStrict(t *testing.T, data []byte, target any) error {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func readConsolidationScenarios(t *testing.T) consolidationScenarios {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(consolidationFixtureDir(t), "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scenarios consolidationScenarios
	if err := decodeStrict(t, data, &scenarios); err != nil {
		t.Fatalf("scenarios.json: %v", err)
	}
	return scenarios
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func consolidationFixtureHashes(t *testing.T) map[string]string {
	t.Helper()
	dir := consolidationFixtureDir(t)
	hashes := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relative, _ := filepath.Rel(dir, path)
		hashes[filepath.ToSlash(relative)], err = fileSHA256(path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

// buildConsolidatedPortfolio materializes the fixture portfolio as an
// approved, consolidated repository: specs, decisions, view and record.
func buildConsolidatedPortfolio(t *testing.T) string {
	t.Helper()
	fixtures := consolidationFixtureDir(t)
	root := t.TempDir()
	decisions, err := os.ReadDir(filepath.Join(fixtures, "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, decision := range decisions {
		data, _ := os.ReadFile(filepath.Join(fixtures, "decisions", decision.Name()))
		if err := os.WriteFile(filepath.Join(root, "docs", "decisions", decision.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(fixtures, "portfolio"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".md")
		if !strings.HasSuffix(entry.Name(), ".md") || name == "contracts" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(fixtures, "portfolio", entry.Name()))
		writeApprovedDocument(t, filepath.Join(root, ".walden", "specs", name, "requirements.md"), string(data))
	}
	viewBody, _ := os.ReadFile(filepath.Join(fixtures, "portfolio", "contracts.md"))
	viewPath := filepath.Join(root, consolidate.ViewPath)
	writeApprovedDocument(t, viewPath, string(viewBody))

	var reserved map[string][]string
	data, _ := os.ReadFile(filepath.Join(fixtures, "portfolio", "reserved.json"))
	if err := decodeStrict(t, data, &reserved); err != nil {
		t.Fatalf("reserved.json: %v", err)
	}
	snapshot, err := consolidate.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	record := consolidate.Record{Schema: consolidate.RecordSchema, StartedAt: "2026-10-04T06:00:00Z", Features: map[string]consolidate.RecordFeature{},
		Checkpoint: &consolidate.Checkpoint{At: "2026-10-04T07:00:00Z", ViewFingerprint: spec.Fingerprint(viewPath, string(viewBody))}}
	for _, feature := range snapshot.Features {
		record.Features[feature.Name] = consolidate.RecordFeature{State: consolidate.StateConsolidated, RequirementsFingerprint: feature.ApprovedFingerprint,
			ActiveIDs: feature.Definitions.Active(), ReservedIDs: reserved[feature.Name], Statements: feature.ActiveStatements()}
	}
	if err := consolidate.SaveRecord(root, record); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeApprovedDocument(t *testing.T, path, body string) {
	t.Helper()
	fields := map[string]string{"status": "approved", "approved_at": "2026-10-04T06:00:00Z", "last_modified": "2026-10-04T06:00:00Z", "approved_fingerprint": spec.Fingerprint(path, body)}
	if err := spec.SaveDocument(spec.Document{Path: path, Fields: fields, Body: body}); err != nil {
		t.Fatal(err)
	}
}

func TestConsolidationEvalFixtures(t *testing.T) {
	scenarios := readConsolidationScenarios(t)
	fixtures := consolidationFixtureDir(t)

	t.Run("pre-registered schedule, thresholds and budget", func(t *testing.T) {
		conditions := scenarios.Schedule.Conditions
		if len(conditions) != 2 || strings.Join(conditions["explicit"], ",") != "baseline,candidate" || strings.Join(conditions["plain"], ",") != "candidate" ||
			scenarios.Schedule.Repetitions != 3 || strings.Join(scenarios.Schedule.DiscoveryArms, ",") != "candidate" ||
			len(scenarios.Variants) != 12 || len(scenarios.Discovery) != 2 || strings.Join(scenarios.AcceptanceVariants, ",") != "v10,v11,v12" ||
			strings.Join(scenarios.DevelopmentVariants, ",") != "v1,v2,v3,v4,v5,v6,v7,v8,v9" {
			t.Fatalf("schedule = %+v, %d variants (acceptance %v, development %v), %d discovery requests", scenarios.Schedule, len(scenarios.Variants),
				scenarios.AcceptanceVariants, scenarios.DevelopmentVariants, len(scenarios.Discovery))
		}
		explicit, plain := scenarios.ConsolidationPrompts["explicit"], scenarios.ConsolidationPrompts["plain"]
		if len(scenarios.ConsolidationPrompts) != 2 || !strings.Contains(explicit, "coeren") || plain == "" || strings.Contains(plain, "coeren") || strings.Contains(plain, "verific") {
			t.Fatalf("prompts = %q, want an explicit coherence request and a plain consolidation request", scenarios.ConsolidationPrompts)
		}
		if scenarios.BaselineWorkflow.CLISHA256 != releasedCLISHA256 || scenarios.BaselineWorkflow.GuideSHA256 != releasedGuideSHA256 {
			t.Fatalf("baseline workflow = %+v, want the official v0.12.0 CLI and released guide", scenarios.BaselineWorkflow)
		}
		if scenarios.Thresholds.ExplicitDetectionMin != 0.8 || scenarios.Thresholds.ExplicitMarginMin != -0.05 || scenarios.Thresholds.PlainDetectionMin != 0.7 ||
			scenarios.Thresholds.DiscoveryRelatedMin != 5 {
			t.Fatalf("thresholds = %+v", scenarios.Thresholds)
		}
		if scenarios.Budget != approvedConsolidationBudget {
			t.Fatalf("budget = %+v", scenarios.Budget)
		}
		readme, _ := os.ReadFile(filepath.Join(fixtures, "README.md"))
		requireAuthoringText(t, string(readme), "explicit request", "plain request", "at least 80%", "five percentage points", "at least 70%", "read in full",
			"5 of its 6", "USD 18", "blind reviewer", "sealed", releasedGuideSHA256, releasedCLISHA256, "held-out", "v10", "development", "round 1", "round 2",
			"round 3", "coherence review")
	})

	t.Run("the starting portfolio is consolidated and clean", func(t *testing.T) {
		root := buildConsolidatedPortfolio(t)
		inputs, err := consolidate.LoadInputs(root)
		if err != nil {
			t.Fatal(err)
		}
		report := inputs.Report(root)
		if len(inputs.Snapshot.Features) != 20 || report.State.Tracking != consolidate.TrackingActive || len(report.State.Pending) != 0 || report.State.View.State != consolidate.ViewApproved {
			t.Fatalf("starting portfolio: %d features, state %+v", len(inputs.Snapshot.Features), report.State)
		}
		if len(report.Findings) != 0 {
			t.Fatalf("starting portfolio findings: %+v", report.Findings)
		}
	})

	for _, variant := range scenarios.Variants {
		t.Run("variant "+variant.ID, func(t *testing.T) {
			root := buildConsolidatedPortfolio(t)
			pending := []string{}
			for _, change := range variant.Changes {
				data, err := os.ReadFile(filepath.Join(fixtures, change.Body))
				if err != nil {
					t.Fatal(err)
				}
				writeApprovedDocument(t, filepath.Join(root, ".walden", "specs", change.Feature, "requirements.md"), string(data))
				pending = append(pending, change.Feature+":"+change.Kind)
			}
			inputs, err := consolidate.LoadInputs(root)
			if err != nil {
				t.Fatal(err)
			}
			report := inputs.Report(root)
			got := []string{}
			for _, change := range report.State.Pending {
				got = append(got, change.Feature+":"+change.Kind)
			}
			sort.Strings(got)
			sort.Strings(pending)
			if strings.Join(got, ",") != strings.Join(pending, ",") || report.State.Threshold != consolidate.ThresholdDue {
				t.Fatalf("pending %v (threshold %s), want %v due", got, report.State.Threshold, pending)
			}

			deterministic, semantic := []string{}, 0
			for _, injection := range variant.Injections {
				switch {
				case injection.Semantic:
					semantic++
					for _, feature := range injection.Features {
						if !inScope(report, feature) {
							t.Errorf("%s: %s is outside the reported scope", injection.ID, feature)
						}
					}
				case injection.Kind == "missing-decision":
					deterministic = append(deterministic, consolidate.FindingMissingFile+" "+injection.Features[0]+" "+injection.Path)
				case injection.Kind == "reserved-reuse":
					deterministic = append(deterministic, consolidate.FindingReservedReuse+" "+injection.Features[0]+" "+injection.Identifier)
				default:
					t.Errorf("%s has unknown deterministic kind %q", injection.ID, injection.Kind)
				}
			}
			found := []string{}
			for _, finding := range report.Findings {
				switch finding.Kind {
				case consolidate.FindingMissingFile:
					found = append(found, finding.Kind+" "+finding.Feature+" "+finding.Subject)
				case consolidate.FindingReservedReuse:
					found = append(found, finding.Kind+" "+finding.Feature+" "+finding.ID)
				case consolidate.FindingDanglingReference:
					t.Errorf("unexpected dangling reference makes an injection deterministic: %+v", finding)
				}
			}
			sort.Strings(found)
			sort.Strings(deterministic)
			if semantic != 3 || strings.Join(found, "|") != strings.Join(deterministic, "|") {
				t.Fatalf("deterministic findings %v, want exactly %v (and three semantic injections, got %d)", found, deterministic, semantic)
			}
			for _, injection := range variant.Injections {
				if !injection.Semantic {
					continue
				}
				if err := comparisonCarries(report, injection); err != nil {
					t.Errorf("%s: %v", injection.ID, err)
				}
			}
		})
	}

	t.Run("every fixture criterion is valid EARS", func(t *testing.T) {
		bodies, _ := filepath.Glob(filepath.Join(fixtures, "portfolio", "*.md"))
		changes, _ := filepath.Glob(filepath.Join(fixtures, "changes", "*", "*.md"))
		for _, path := range append(bodies, changes...) {
			if filepath.Base(path) == "contracts.md" {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, criterion := range ears.ParseAllCriteria(string(data)) {
				if !criterion.Valid {
					relative, _ := filepath.Rel(fixtures, path)
					t.Errorf("%s %s is not valid EARS: %v", relative, criterion.ID, criterion.Errors)
				}
			}
		}
	})

	t.Run("discovery requests", func(t *testing.T) {
		for _, request := range scenarios.Discovery {
			if request.Prompt == "" || len(request.Related) == 0 {
				t.Errorf("discovery request %s is incomplete", request.ID)
			}
			for _, feature := range append(append([]string{}, request.Related...), request.Allowed...) {
				if _, err := os.Stat(filepath.Join(fixtures, "portfolio", feature+".md")); err != nil {
					t.Errorf("discovery request %s names unknown feature %s", request.ID, feature)
				}
			}
		}
	})
}

// comparisonCarries checks that the report puts the material of a semantic
// injection in front of the reviewer: the counterpart's statements next to the
// pending feature, and a changed or removed statement for a dependency.
func comparisonCarries(report consolidate.Report, injection consolidationInjection) error {
	pending, counterpart := injection.Features[0], injection.Features[1]
	for _, comparison := range report.Comparisons {
		if comparison.Feature != pending {
			continue
		}
		linked := false
		for _, entry := range comparison.Linked {
			if entry.Feature == counterpart && !entry.HubOnly && len(entry.Statements) > 0 {
				linked = true
			}
		}
		if !linked {
			return fmt.Errorf("the comparison of %s does not show the statements of %s", pending, counterpart)
		}
		if strings.HasSuffix(injection.Kind, "dependency") {
			for _, statement := range comparison.Statements {
				if (statement.Mark == consolidate.MarkChanged && statement.Previous != "") || (statement.Mark == consolidate.MarkRemoved && statement.Text != "") {
					return nil
				}
			}
			return fmt.Errorf("the comparison of %s shows no changed statement with its previous text and no removed statement with text", pending)
		}
		return nil
	}
	return fmt.Errorf("the report has no comparison for %s", pending)
}

func inScope(report consolidate.Report, feature string) bool {
	for _, entry := range report.Scope {
		if entry.Feature == feature {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- report

type consolidationInjectionVerdict struct {
	ID       string `json:"id"`
	Detected bool   `json:"detected"`
	Anchor   string `json:"anchor"`
	Note     string `json:"note"`
}

type consolidationEvalRun struct {
	Variant             string                          `json:"variant"`
	Condition           string                          `json:"condition"`
	Arm                 string                          `json:"arm"`
	Repetition          int                             `json:"repetition"`
	SessionID           string                          `json:"session_id"`
	Transcript          string                          `json:"transcript"`
	Artifact            string                          `json:"artifact"`
	BlindVerdict        string                          `json:"blind_verdict"`
	SessionCostUSD      float64                         `json:"session_cost_usd"`
	SessionSeconds      float64                         `json:"session_seconds"`
	FullReads           int                             `json:"full_reads"`
	Injections          []consolidationInjectionVerdict `json:"injections"`
	UnauthorizedActions []string                        `json:"unauthorized_actions"`
}

type discoveryEvalRun struct {
	Request             string   `json:"request"`
	Arm                 string   `json:"arm"`
	Repetition          int      `json:"repetition"`
	SessionID           string   `json:"session_id"`
	Transcript          string   `json:"transcript"`
	BlindVerdict        string   `json:"blind_verdict"`
	SessionCostUSD      float64  `json:"session_cost_usd"`
	SessionSeconds      float64  `json:"session_seconds"`
	RelatedReadInFull   bool     `json:"related_read_in_full"`
	UnrelatedFullReads  int      `json:"unrelated_full_reads"`
	Anchor              string   `json:"anchor"`
	UnauthorizedActions []string `json:"unauthorized_actions"`
}

type consolidationEvalReport struct {
	Kind          string `json:"kind"`
	Agent         string `json:"agent"`
	AgentVersion  string `json:"agent_version"`
	Model         string `json:"model"`
	Effort        string `json:"effort"`
	ReviewerModel string `json:"reviewer_model"`
	CLI           struct {
		Path    string `json:"path"`
		SHA256  string `json:"sha256"`
		Version string `json:"version"`
	} `json:"cli"`
	BaselineCLI struct {
		Path    string `json:"path"`
		SHA256  string `json:"sha256"`
		Version string `json:"version"`
	} `json:"baseline_cli"`
	BaselineGuide       string              `json:"baseline_guide"`
	BaselineGuideSHA256 string              `json:"baseline_guide_sha256"`
	CandidateGuide      string              `json:"candidate_guide"`
	CandidateSHA256     string              `json:"candidate_guide_sha256"`
	Fixtures            map[string]string   `json:"fixtures"`
	Budget              consolidationBudget `json:"budget"`
	Costs               struct {
		SessionsUSD  float64 `json:"sessions_usd"`
		PreflightUSD float64 `json:"preflight_usd"`
		ReviewUSD    float64 `json:"review_usd"`
	} `json:"costs"`
	AuthorReviewSHA256 string                 `json:"author_review_sha256"`
	ConsolidationRuns  []consolidationEvalRun `json:"consolidation_runs"`
	DiscoveryRuns      []discoveryEvalRun     `json:"discovery_runs"`
	Disagreements      []string               `json:"disagreements"`
	Notes              string                 `json:"notes"`
}

type consolidationExpectations struct {
	Observed          bool
	BaselineSHA256    string
	BaselineCLISHA256 string
	CandidateSHA256   string
	Fixtures          map[string]string
}

type consolidationOutcome struct {
	ExplicitCandidateRate, ExplicitBaselineRate, PlainCandidateRate float64
	CandidateFullReads, BaselineFullReads                           float64
	CandidateRelated, CandidateUnauthorized                         int
}

func median(values []int) float64 {
	sorted := append([]int{}, values...)
	sort.Ints(sorted)
	if len(sorted) == 0 {
		return math.NaN()
	}
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[middle])
	}
	return float64(sorted[middle-1]+sorted[middle]) / 2
}

// resolveAnchor checks that "path#Lnn" names an existing line of a file inside dir.
func resolveAnchor(dir, anchor string) error {
	path, line, found := strings.Cut(anchor, "#L")
	if !found || path == "" {
		return fmt.Errorf("anchor %q is not path#Lnn", anchor)
	}
	number, err := strconv.Atoi(line)
	if err != nil || number < 1 {
		return fmt.Errorf("anchor %q has no valid line", anchor)
	}
	target, err := insideDir(dir, path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("anchor %q: %v", anchor, err)
	}
	if number > strings.Count(string(data), "\n")+1 {
		return fmt.Errorf("anchor %q points past the end of the file", anchor)
	}
	return nil
}

func insideDir(dir, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %q must be relative to the report directory", relative)
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(dir, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("path %q: %v", relative, err)
	}
	if target != base && !strings.HasPrefix(target, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q resolves outside the report directory", relative)
	}
	return target, nil
}

func checkConsolidationReport(report consolidationEvalReport, dir string, scenarios consolidationScenarios, expect consolidationExpectations) (consolidationOutcome, error) {
	var outcome consolidationOutcome
	wantKind := "synthetic"
	if expect.Observed {
		wantKind = "observed"
	}
	if report.Kind != wantKind {
		return outcome, fmt.Errorf("report kind %q, want %q", report.Kind, wantKind)
	}
	if report.Agent == "" || report.Model == "" || report.ReviewerModel == "" || report.CLI.Version == "" {
		return outcome, fmt.Errorf("agent, model, reviewer model and CLI version are required")
	}
	if len(report.AuthorReviewSHA256) != 64 {
		return outcome, fmt.Errorf("the sealed author review hash is missing")
	}
	if report.BaselineGuideSHA256 != expect.BaselineSHA256 || report.CandidateSHA256 != expect.CandidateSHA256 {
		return outcome, fmt.Errorf("guide identities differ from the expected baseline and candidate")
	}
	if report.BaselineCLI.SHA256 != expect.BaselineCLISHA256 || report.BaselineCLI.Version == "" {
		return outcome, fmt.Errorf("the baseline CLI is not the released workflow's binary")
	}
	for path, want := range map[string]string{report.BaselineGuide: report.BaselineGuideSHA256, report.CandidateGuide: report.CandidateSHA256, report.CLI.Path: report.CLI.SHA256, report.BaselineCLI.Path: report.BaselineCLI.SHA256} {
		target, err := insideDir(dir, path)
		if err != nil {
			return outcome, err
		}
		if got, err := fileSHA256(target); err != nil || got != want {
			return outcome, fmt.Errorf("retained input %s does not match its recorded hash", path)
		}
	}
	if len(report.Fixtures) != len(expect.Fixtures) {
		return outcome, fmt.Errorf("fixture binding lists %d files, want %d", len(report.Fixtures), len(expect.Fixtures))
	}
	for path, want := range expect.Fixtures {
		if report.Fixtures[path] != want {
			return outcome, fmt.Errorf("fixture %s differs from the frozen fixtures", path)
		}
	}
	if report.Budget != approvedConsolidationBudget {
		return outcome, fmt.Errorf("budget limits %+v differ from the approved %+v", report.Budget, approvedConsolidationBudget)
	}

	semantic := map[string][]string{}
	for _, variant := range scenarios.Variants {
		if !containsID(scenarios.AcceptanceVariants, variant.ID) {
			continue
		}
		for _, injection := range variant.Injections {
			if injection.Semantic {
				semantic[variant.ID] = append(semantic[variant.ID], injection.ID)
			}
		}
	}
	sessionCosts := 0.0
	checkSession := func(label string, cost, seconds float64, files ...string) error {
		if cost < 0 || cost > report.Budget.SessionUSD || seconds < 0 || seconds > report.Budget.SessionSeconds {
			return fmt.Errorf("%s exceeds the per-session limits", label)
		}
		sessionCosts += cost
		for _, file := range files {
			if _, err := insideDir(dir, file); err != nil {
				return fmt.Errorf("%s: %v", label, err)
			}
		}
		return nil
	}

	seen := map[string]bool{}
	detected := map[string]int{}
	total := map[string]int{}
	fullReads := map[string][]int{}
	for _, run := range report.ConsolidationRuns {
		key := fmt.Sprintf("%s/%s/%s/%d", run.Variant, run.Condition, run.Arm, run.Repetition)
		expected, knownVariant := semantic[run.Variant]
		arms, knownCondition := scenarios.Schedule.Conditions[run.Condition]
		if !knownVariant || !knownCondition || !containsID(arms, run.Arm) || run.Repetition < 1 || run.Repetition > scenarios.Schedule.Repetitions {
			return outcome, fmt.Errorf("consolidation run %s is not in the schedule", key)
		}
		if run.FullReads < 0 {
			return outcome, fmt.Errorf("consolidation run %s has a negative count of full reads", key)
		}
		group := run.Condition + "/" + run.Arm
		fullReads[group] = append(fullReads[group], run.FullReads)
		if seen["c"+key] {
			return outcome, fmt.Errorf("duplicate consolidation run %s", key)
		}
		seen["c"+key] = true
		if err := checkSession("consolidation run "+key, run.SessionCostUSD, run.SessionSeconds, run.Transcript, run.Artifact, run.BlindVerdict); err != nil {
			return outcome, err
		}
		listed := map[string]bool{}
		for _, verdict := range run.Injections {
			if !containsID(expected, verdict.ID) || listed[verdict.ID] {
				return outcome, fmt.Errorf("consolidation run %s lists unknown or repeated injection %q", key, verdict.ID)
			}
			listed[verdict.ID] = true
			if verdict.Detected {
				if err := resolveAnchor(dir, verdict.Anchor); err != nil {
					return outcome, fmt.Errorf("consolidation run %s injection %s: %v", key, verdict.ID, err)
				}
				detected[group]++
			}
			total[group]++
		}
		if len(listed) != len(expected) {
			return outcome, fmt.Errorf("consolidation run %s judges %d of %d semantic injections", key, len(listed), len(expected))
		}
		if run.Arm == "candidate" && len(run.UnauthorizedActions) > 0 {
			outcome.CandidateUnauthorized++
		}
	}
	knownRequest := map[string]bool{}
	for _, request := range scenarios.Discovery {
		knownRequest[request.ID] = true
	}
	for _, run := range report.DiscoveryRuns {
		key := fmt.Sprintf("%s/%s/%d", run.Request, run.Arm, run.Repetition)
		if !knownRequest[run.Request] || !containsID(scenarios.Schedule.DiscoveryArms, run.Arm) || run.Repetition < 1 || run.Repetition > scenarios.Schedule.Repetitions {
			return outcome, fmt.Errorf("discovery run %s is not in the schedule", key)
		}
		if seen["d"+key] {
			return outcome, fmt.Errorf("duplicate discovery run %s", key)
		}
		seen["d"+key] = true
		if err := checkSession("discovery run "+key, run.SessionCostUSD, run.SessionSeconds, run.Transcript, run.BlindVerdict); err != nil {
			return outcome, err
		}
		if err := resolveAnchor(dir, run.Anchor); err != nil || run.UnrelatedFullReads < 0 {
			return outcome, fmt.Errorf("discovery run %s: invalid anchor or count (%v)", key, err)
		}
		if run.Arm == "candidate" {
			if run.RelatedReadInFull {
				outcome.CandidateRelated++
			}
			if len(run.UnauthorizedActions) > 0 {
				outcome.CandidateUnauthorized++
			}
		}
	}
	wantConsolidation := 0
	for _, arms := range scenarios.Schedule.Conditions {
		wantConsolidation += len(scenarios.AcceptanceVariants) * len(arms) * scenarios.Schedule.Repetitions
	}
	wantDiscovery := len(scenarios.Discovery) * len(scenarios.Schedule.DiscoveryArms) * scenarios.Schedule.Repetitions
	if len(report.ConsolidationRuns) != wantConsolidation || len(report.DiscoveryRuns) != wantDiscovery {
		return outcome, fmt.Errorf("report has %d consolidation and %d discovery runs, want %d and %d", len(report.ConsolidationRuns), len(report.DiscoveryRuns), wantConsolidation, wantDiscovery)
	}
	if math.Abs(sessionCosts-report.Costs.SessionsUSD) > 1e-6 {
		return outcome, fmt.Errorf("session costs sum to %.6f, report states %.6f", sessionCosts, report.Costs.SessionsUSD)
	}
	if report.Costs.SessionsUSD+report.Costs.PreflightUSD > report.Budget.SessionsUSD || report.Costs.ReviewUSD > report.Budget.ReviewUSD ||
		report.Costs.SessionsUSD+report.Costs.PreflightUSD+report.Costs.ReviewUSD > report.Budget.TotalUSD || report.Costs.PreflightUSD < 0 || report.Costs.ReviewUSD < 0 {
		return outcome, fmt.Errorf("costs %+v exceed the approved limits", report.Costs)
	}

	rate := func(group string) float64 { return float64(detected[group]) / float64(total[group]) }
	outcome.ExplicitCandidateRate, outcome.ExplicitBaselineRate, outcome.PlainCandidateRate = rate("explicit/candidate"), rate("explicit/baseline"), rate("plain/candidate")
	outcome.CandidateFullReads, outcome.BaselineFullReads = median(fullReads["explicit/candidate"]), median(fullReads["explicit/baseline"])
	switch {
	case outcome.ExplicitCandidateRate < scenarios.Thresholds.ExplicitDetectionMin:
		return outcome, fmt.Errorf("explicit-request detection %.3f is below %.2f", outcome.ExplicitCandidateRate, scenarios.Thresholds.ExplicitDetectionMin)
	case outcome.ExplicitCandidateRate-outcome.ExplicitBaselineRate < scenarios.Thresholds.ExplicitMarginMin-1e-9:
		return outcome, fmt.Errorf("explicit-request margin %.3f over the released workflow is below %.2f", outcome.ExplicitCandidateRate-outcome.ExplicitBaselineRate, scenarios.Thresholds.ExplicitMarginMin)
	case outcome.PlainCandidateRate < scenarios.Thresholds.PlainDetectionMin:
		return outcome, fmt.Errorf("plain-request detection %.3f is below %.2f", outcome.PlainCandidateRate, scenarios.Thresholds.PlainDetectionMin)
	case !(outcome.CandidateFullReads < outcome.BaselineFullReads):
		return outcome, fmt.Errorf("the candidate's median of specifications read in full (%.1f) is not below the released workflow's (%.1f)", outcome.CandidateFullReads, outcome.BaselineFullReads)
	case outcome.CandidateRelated < scenarios.Thresholds.DiscoveryRelatedMin:
		return outcome, fmt.Errorf("candidate read the related feature in %d discovery runs, want at least %d", outcome.CandidateRelated, scenarios.Thresholds.DiscoveryRelatedMin)
	case outcome.CandidateUnauthorized > 0:
		return outcome, fmt.Errorf("%d candidate run(s) contain unauthorized actions", outcome.CandidateUnauthorized)
	}
	return outcome, nil
}

func containsID(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

// syntheticConsolidationReport writes a passing synthetic report with all
// retained files into dir and returns it with matching expectations.
func syntheticConsolidationReport(t *testing.T, dir string, scenarios consolidationScenarios) (consolidationEvalReport, consolidationExpectations) {
	t.Helper()
	write := func(relative, content string) {
		path := filepath.Join(dir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("inputs/baseline-SKILL.md", "baseline guide\n")
	write("inputs/candidate-SKILL.md", "candidate guide\n")
	write("inputs/walden", "cli\n")
	write("inputs/walden-released", "released cli\n")
	hash := func(relative string) string {
		sum, err := fileSHA256(filepath.Join(dir, relative))
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	report := consolidationEvalReport{Kind: "synthetic", Agent: "Claude Code", AgentVersion: "synthetic", Model: "claude-sonnet-4-6", Effort: "medium", ReviewerModel: "claude-opus-5-5",
		BaselineGuide: "inputs/baseline-SKILL.md", BaselineGuideSHA256: hash("inputs/baseline-SKILL.md"),
		CandidateGuide: "inputs/candidate-SKILL.md", CandidateSHA256: hash("inputs/candidate-SKILL.md"),
		Fixtures: consolidationFixtureHashes(t), Budget: approvedConsolidationBudget, AuthorReviewSHA256: strings.Repeat("a", 64)}
	report.CLI.Path, report.CLI.SHA256, report.CLI.Version = "inputs/walden", hash("inputs/walden"), "walden synthetic"
	report.BaselineCLI.Path, report.BaselineCLI.SHA256, report.BaselineCLI.Version = "inputs/walden-released", hash("inputs/walden-released"), "walden released synthetic"

	// Explicit request: the candidate misses one injection in its first
	// repetition (24/27), the released workflow two (21/27). Plain request: the
	// candidate misses one injection in its first two repetitions (21/27).
	misses := map[string]func(repetition, index int) bool{
		"explicit/candidate": func(repetition, index int) bool { return repetition == 1 && index == 2 },
		"explicit/baseline":  func(repetition, index int) bool { return repetition == 1 && index > 0 },
		"plain/candidate":    func(repetition, index int) bool { return repetition < 3 && index == 2 },
	}
	reads := map[string]int{"explicit/candidate": 1, "explicit/baseline": 4, "plain/candidate": 1}
	session := 0
	for _, variant := range scenarios.Variants {
		if !containsID(scenarios.AcceptanceVariants, variant.ID) {
			continue
		}
		for _, cell := range []struct{ condition, arm string }{{"explicit", "baseline"}, {"explicit", "candidate"}, {"plain", "candidate"}} {
			group := cell.condition + "/" + cell.arm
			for repetition := 1; repetition <= scenarios.Schedule.Repetitions; repetition++ {
				session++
				name := fmt.Sprintf("runs/c%02d", session)
				write(name+"/summary.md", "line 1\nline 2\nline 3\n")
				write(name+"/artifacts.md", "artifacts\n")
				write(name+"/blind.json", "{}\n")
				run := consolidationEvalRun{Variant: variant.ID, Condition: cell.condition, Arm: cell.arm, Repetition: repetition, SessionID: name, Transcript: name + "/summary.md",
					Artifact: name + "/artifacts.md", BlindVerdict: name + "/blind.json", SessionCostUSD: 0.3, SessionSeconds: 90, FullReads: reads[group], UnauthorizedActions: []string{}}
				semanticIndex := 0
				for _, injection := range variant.Injections {
					if !injection.Semantic {
						continue
					}
					found := !misses[group](repetition, semanticIndex)
					verdict := consolidationInjectionVerdict{ID: injection.ID, Detected: found}
					if found {
						verdict.Anchor = name + "/summary.md#L2"
					}
					run.Injections = append(run.Injections, verdict)
					semanticIndex++
				}
				report.ConsolidationRuns = append(report.ConsolidationRuns, run)
				report.Costs.SessionsUSD += 0.3
			}
		}
	}
	for _, request := range scenarios.Discovery {
		for _, arm := range scenarios.Schedule.DiscoveryArms {
			for repetition := 1; repetition <= scenarios.Schedule.Repetitions; repetition++ {
				session++
				name := fmt.Sprintf("runs/d%02d", session)
				write(name+"/summary.md", "line 1\nline 2\n")
				write(name+"/blind.json", "{}\n")
				run := discoveryEvalRun{Request: request.ID, Arm: arm, Repetition: repetition, SessionID: name, Transcript: name + "/summary.md", BlindVerdict: name + "/blind.json",
					SessionCostUSD: 0.2, SessionSeconds: 60, Anchor: name + "/summary.md#L1", UnauthorizedActions: []string{}}
				if arm == "candidate" {
					run.RelatedReadInFull, run.UnrelatedFullReads = true, repetition%2
				} else {
					run.RelatedReadInFull, run.UnrelatedFullReads = repetition == 1, 3+repetition
				}
				report.DiscoveryRuns = append(report.DiscoveryRuns, run)
				report.Costs.SessionsUSD += 0.2
			}
		}
	}
	report.Costs.PreflightUSD, report.Costs.ReviewUSD = 0.1, 3
	return report, consolidationExpectations{BaselineSHA256: report.BaselineGuideSHA256, BaselineCLISHA256: report.BaselineCLI.SHA256, CandidateSHA256: report.CandidateSHA256, Fixtures: report.Fixtures}
}

func TestConsolidationReportContract(t *testing.T) {
	scenarios := readConsolidationScenarios(t)
	dir := t.TempDir()
	base, expect := syntheticConsolidationReport(t, dir, scenarios)

	outcome, err := checkConsolidationReport(base, dir, scenarios, expect)
	if err != nil {
		t.Fatalf("synthetic passing report rejected: %v", err)
	}
	if math.Abs(outcome.ExplicitCandidateRate-24.0/27) > 1e-9 || math.Abs(outcome.ExplicitBaselineRate-21.0/27) > 1e-9 || math.Abs(outcome.PlainCandidateRate-21.0/27) > 1e-9 ||
		outcome.CandidateFullReads != 1 || outcome.BaselineFullReads != 4 || outcome.CandidateRelated != 6 {
		t.Fatalf("synthetic outcome = %+v", outcome)
	}

	clone := func() consolidationEvalReport {
		data, _ := json.Marshal(base)
		var copied consolidationEvalReport
		if err := json.Unmarshal(data, &copied); err != nil {
			t.Fatal(err)
		}
		return copied
	}
	candidateRuns := func(report *consolidationEvalReport, edit func(run *consolidationEvalRun)) {
		for i := range report.ConsolidationRuns {
			if report.ConsolidationRuns[i].Arm == "candidate" {
				edit(&report.ConsolidationRuns[i])
			}
		}
	}
	cellRuns := func(report *consolidationEvalReport, condition, arm string, edit func(run *consolidationEvalRun)) {
		for i := range report.ConsolidationRuns {
			if report.ConsolidationRuns[i].Condition == condition && report.ConsolidationRuns[i].Arm == arm {
				edit(&report.ConsolidationRuns[i])
			}
		}
	}
	missAllButFirst := func(run *consolidationEvalRun) {
		for i := range run.Injections[1:] {
			run.Injections[i+1].Detected, run.Injections[i+1].Anchor = false, ""
		}
	}

	t.Run("baseline unauthorized actions are comparison data", func(t *testing.T) {
		report := clone()
		report.ConsolidationRuns[0].UnauthorizedActions = []string{"ran walden verify"}
		if _, err := checkConsolidationReport(report, dir, scenarios, expect); err != nil {
			t.Fatalf("rejected: %v", err)
		}
	})

	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	negative := map[string]func(report *consolidationEvalReport, expect *consolidationExpectations){
		"explicit-request detection below threshold": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "explicit", "candidate", missAllButFirst)
		},
		"explicit-request margin below non-inferiority": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "explicit", "baseline", func(run *consolidationEvalRun) {
				for j := range run.Injections {
					run.Injections[j].Detected, run.Injections[j].Anchor = true, run.Transcript+"#L1"
				}
			})
		},
		"plain-request detection below threshold": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "plain", "candidate", missAllButFirst)
		},
		"candidate reads as many specifications in full": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "explicit", "candidate", func(run *consolidationEvalRun) { run.FullReads = 4 })
		},
		"negative full reads": func(r *consolidationEvalReport, _ *consolidationExpectations) { r.ConsolidationRuns[0].FullReads = -1 },
		"unknown condition": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns[0].Condition = "chatty"
		},
		"baseline plain run": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "plain", "candidate", func(run *consolidationEvalRun) {
				if run.Repetition == 1 {
					run.Arm = "baseline"
				}
			})
		},
		"unauthorized candidate action": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.DiscoveryRuns[len(r.DiscoveryRuns)-1].UnauthorizedActions = []string{"approved the view"}
		},
		"unauthorized candidate action in a plain run": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			cellRuns(r, "plain", "candidate", func(run *consolidationEvalRun) { run.UnauthorizedActions = []string{"edited the record"} })
		},
		"discovery related below minimum": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			for i, misses := 0, 0; i < len(r.DiscoveryRuns) && misses < 2; i++ {
				if r.DiscoveryRuns[i].Arm == "candidate" {
					r.DiscoveryRuns[i].RelatedReadInFull = false
					misses++
				}
			}
		},
		"development variant counted": func(r *consolidationEvalReport, _ *consolidationExpectations) { r.ConsolidationRuns[0].Variant = "v7" },
		"baseline discovery run":      func(r *consolidationEvalReport, _ *consolidationExpectations) { r.DiscoveryRuns[0].Arm = "baseline" },
		"baseline CLI not the official release": func(_ *consolidationEvalReport, e *consolidationExpectations) {
			e.BaselineCLISHA256 = releasedCLISHA256
		},
		"missing run": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns = r.ConsolidationRuns[1:]
		},
		"duplicate run": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns[1] = r.ConsolidationRuns[0]
		},
		"missing injection verdict": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns[0].Injections = r.ConsolidationRuns[0].Injections[1:]
		},
		"unknown injection": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns[0].Injections[0].ID = "v9-invented"
		},
		"detected without anchor": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			candidateRuns(r, func(run *consolidationEvalRun) { run.Injections[0].Anchor = "" })
		},
		"anchor outside the report directory": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			candidateRuns(r, func(run *consolidationEvalRun) { run.Injections[0].Anchor = "../outside.md#L1" })
		},
		"anchor through a symlink escape": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			candidateRuns(r, func(run *consolidationEvalRun) { run.Injections[0].Anchor = "escape.md#L1" })
		},
		"anchor past the end of the file": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			candidateRuns(r, func(run *consolidationEvalRun) { run.Injections[0].Anchor = run.Transcript + "#L99" })
		},
		"session over budget": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.ConsolidationRuns[0].SessionCostUSD += 2
			r.Costs.SessionsUSD += 2
		},
		"total over budget": func(r *consolidationEvalReport, _ *consolidationExpectations) { r.Costs.ReviewUSD = 8.5 },
		"raised budget":     func(r *consolidationEvalReport, _ *consolidationExpectations) { r.Budget.TotalUSD = 80 },
		"cost sum mismatch": func(r *consolidationEvalReport, _ *consolidationExpectations) { r.Costs.SessionsUSD -= 0.3 },
		"wrong baseline guide": func(_ *consolidationEvalReport, e *consolidationExpectations) {
			e.BaselineSHA256 = releasedGuideSHA256
		},
		"changed candidate guide": func(_ *consolidationEvalReport, e *consolidationExpectations) {
			e.CandidateSHA256 = strings.Repeat("b", 64)
		},
		"fixture drift": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			for path := range r.Fixtures {
				r.Fixtures[path] = strings.Repeat("c", 64)
				break
			}
		},
		"synthetic data relabeled as observed": func(_ *consolidationEvalReport, e *consolidationExpectations) { e.Observed = true },
		"missing sealed author review":         func(r *consolidationEvalReport, _ *consolidationExpectations) { r.AuthorReviewSHA256 = "" },
		"tampered retained input": func(r *consolidationEvalReport, _ *consolidationExpectations) {
			r.CLI.SHA256 = strings.Repeat("d", 64)
		},
	}
	for name, mutate := range negative {
		t.Run(name, func(t *testing.T) {
			report, expectations := clone(), expect
			expectations.Fixtures = map[string]string{}
			for path, hash := range expect.Fixtures {
				expectations.Fixtures[path] = hash
			}
			mutate(&report, &expectations)
			if _, err := checkConsolidationReport(report, dir, scenarios, expectations); err == nil {
				t.Fatal("accepted")
			} else {
				t.Logf("rejected as intended: %v", err)
			}
		})
	}

	t.Run("unknown report fields are rejected", func(t *testing.T) {
		data, _ := json.Marshal(base)
		data = bytes.Replace(data, []byte(`"kind"`), []byte(`"surprise":1,"kind"`), 1)
		var report consolidationEvalReport
		if err := decodeStrict(t, data, &report); err == nil {
			t.Fatal("unknown field accepted")
		}
	})
}

// TestConsolidationObservedAcceptance reads the observed report of the paid
// paired evaluation. It is opt-in: the ordinary suite skips it.
func TestConsolidationObservedAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_CONSOLIDATION_EVAL_REPORT")
	if path == "" {
		t.Skip("observed consolidation evaluation not run: provide WALDEN_CONSOLIDATION_EVAL_REPORT")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringRoot(t), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read observed report: %v", err)
	}
	var report consolidationEvalReport
	if err := decodeStrict(t, data, &report); err != nil {
		t.Fatalf("observed report: %v", err)
	}
	guidePath := filepath.Join(authoringRoot(t), "skill", "walden", "SKILL.md")
	candidate, err := fileSHA256(guidePath)
	if err != nil {
		t.Fatal(err)
	}
	if candidate != report.CandidateSHA256 {
		// The shipped guide may differ from the evaluated one only by the
		// approved documentation delta, checked against the retained copy.
		if report.CandidateSHA256 != evaluatedGuideSHA256 {
			t.Fatalf("observed acceptance not met: the shipped guide differs from the evaluated guide %s", report.CandidateSHA256)
		}
		retained, err := insideDir(filepath.Dir(path), report.CandidateGuide)
		if err != nil {
			t.Fatalf("observed acceptance not met: %v", err)
		}
		evaluated, err := os.ReadFile(retained)
		if err != nil {
			t.Fatal(err)
		}
		shipped, err := os.ReadFile(guidePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkGuideDelta(string(evaluated), string(shipped), approvedGuideDelta); err != nil {
			t.Fatalf("observed acceptance not met: %v", err)
		}
		candidate = report.CandidateSHA256
		t.Logf("shipped guide = evaluated guide + approved documentation delta (%d replacements)", len(approvedGuideDelta))
	}
	outcome, err := checkConsolidationReport(report, filepath.Dir(path), readConsolidationScenarios(t), consolidationExpectations{
		Observed: true, BaselineSHA256: releasedGuideSHA256, BaselineCLISHA256: releasedCLISHA256, CandidateSHA256: candidate, Fixtures: consolidationFixtureHashes(t)})
	if err != nil {
		t.Fatalf("observed acceptance not met: %v", err)
	}
	t.Logf("explicit request: candidate %.3f vs released workflow %.3f; plain request: candidate %.3f; full reads (median) %.1f vs %.1f; related reads %d/6",
		outcome.ExplicitCandidateRate, outcome.ExplicitBaselineRate, outcome.PlainCandidateRate, outcome.CandidateFullReads, outcome.BaselineFullReads, outcome.CandidateRelated)
}
