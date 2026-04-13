package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/ears"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/workflow"
)

var (
	statusValues           = map[string]struct{}{"draft": {}, "in-review": {}, "approved": {}}
	checkboxPattern        = regexp.MustCompile(`(?m)^- \[[ x]\] \d+\.`)
	subtaskPattern         = regexp.MustCompile(`^(\s*)- \[[ x]\] (\d+(?:\.\d+)?)\b`)
	requirementHeader      = regexp.MustCompile(`(?m)^### (R\d+)\b`)
	acceptanceIDPattern    = regexp.MustCompile("`(R\\d+\\.AC\\d+)`")
	nfrIDPattern           = regexp.MustCompile("`(NFR\\d+)`")
	constraintIDPattern    = regexp.MustCompile("`(C\\d+)`")
	backtickIDPattern      = regexp.MustCompile("`((?:R\\d+(?:\\.AC\\d+)?)|(?:NFR\\d+)|(?:C\\d+))`")
	coverageRowPattern     = regexp.MustCompile("(?m)^\\| `((?:R\\d+)|(?:NFR\\d+))` \\|")
	requiredSectionsDesign = []string{
		"## Architecture",
		"## Options Considered",
		"## Simplicity And Elegance Review",
		"## Failure Modes And Tradeoffs",
		"## Verification Plan",
		"## Requirement Coverage",
	}
)

// Result is the deterministic outcome of a validation run.
type Result struct {
	Feature         string
	SpecDir         string
	Valid           bool
	Message         string
	Scope           Scope
	ValidatedPhases []string
	SkippedPhases   []string
	Warnings        []string
	EARSResults     []EARSCriterion
	Coverage         *CoverageReport
	EARSDistribution *EARSDistribution
}

// EARSDistribution reports the count of criteria per EARS form.
type EARSDistribution struct {
	Ubiquitous  int `json:"ubiquitous"`
	EventDriven int `json:"event_driven"`
	StateDriven int `json:"state_driven"`
	Optional    int `json:"optional"`
	Unwanted    int `json:"unwanted"`
	Complex     int `json:"complex"`
	Total       int `json:"total"`
}

// CoverageReport reports task reference and proof reference coverage separately.
type CoverageReport struct {
	TaskReferenceCoverage  CoverageStatus `json:"task_reference_coverage"`
	ProofReferenceCoverage CoverageStatus `json:"proof_reference_coverage"`
}

// CoverageStatus reports whether coverage is complete and which IDs are missing.
type CoverageStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing,omitempty"`
}

// EARSCriterion reports the EARS parse result for one acceptance criterion.
type EARSCriterion struct {
	ID       string   `json:"id"`
	Form     string   `json:"form"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Scope controls which parts of a feature spec are validated.
type Scope string

const (
	ScopeCurrentPhase Scope = "current-phase"
	ScopeFullSpec     Scope = "full-spec"
)

type validationPlan struct {
	scope                Scope
	validateRequirements bool
	validateDesign       bool
	validateTasks        bool
}

// ValidateFeature validates the current phase and approved upstream documents.
func ValidateFeature(root, rawFeature string) (Result, error) {
	return ValidateFeatureWithScope(root, rawFeature, ScopeCurrentPhase)
}

// ValidateFeatureWithScope ports the current Walden spec validator into Go.
func ValidateFeatureWithScope(root, rawFeature string, scope Scope) (Result, error) {
	featureName, err := spec.NormalizeFeatureName(rawFeature)
	if err != nil {
		return Result{Valid: false, Message: fmt.Sprintf("INVALID: %v", err)}, nil
	}

	specDir := filepath.Join(root, ".walden", "specs", featureName)
	if _, err := os.Stat(specDir); err != nil {
		if os.IsNotExist(err) {
			return Result{
				Feature: featureName,
				SpecDir: specDir,
				Valid:   false,
				Message: fmt.Sprintf("INVALID: feature spec not found: %s", specDir),
				Scope:   scope,
			}, nil
		}
		return Result{}, fmt.Errorf("inspect spec dir: %w", err)
	}

	feature, err := spec.LoadFeature(root, featureName)
	if err != nil {
		return Result{}, err
	}

	if !feature.Requirements.Exists && !feature.Design.Exists && !feature.Tasks.Exists {
		return Result{
			Feature: featureName,
			SpecDir: specDir,
			Valid:   false,
			Message: fmt.Sprintf("INVALID: no spec files found in %s", specDir),
			Scope:   scope,
		}, nil
	}

	plan, err := resolveValidationPlan(feature, scope)
	if err != nil {
		return Result{}, err
	}

	if err := validateDocuments(feature, plan); err != nil {
		return Result{
			Feature:         featureName,
			SpecDir:         specDir,
			Valid:           false,
			Message:         fmt.Sprintf("INVALID: %v", err),
			Scope:           scope,
			ValidatedPhases: plan.validatedPhases(),
			SkippedPhases:   plan.skippedPhases(),
		}, nil
	}

	warnings := collectWarnings(feature, plan)
	earsResults := collectEARSResults(feature, plan)
	coverage := collectCoverageReport(feature, plan)
	earsDist := collectEARSDistribution(earsResults)
	qualitySignals := collectQualitySignals(feature, plan, earsResults)
	warnings = append(warnings, qualitySignals...)

	return Result{
		Feature:          featureName,
		SpecDir:          specDir,
		Valid:            true,
		Message:          fmt.Sprintf("VALID: .walden/specs/%s", featureName),
		Scope:            scope,
		ValidatedPhases:  plan.validatedPhases(),
		SkippedPhases:    plan.skippedPhases(),
		Warnings:         warnings,
		EARSResults:      earsResults,
		Coverage:         coverage,
		EARSDistribution: earsDist,
	}, nil
}

func collectEARSDistribution(results []EARSCriterion) *EARSDistribution {
	if len(results) == 0 {
		return &EARSDistribution{}
	}
	dist := &EARSDistribution{Total: len(results)}
	for _, c := range results {
		switch c.Form {
		case ears.FormUbiquitous:
			dist.Ubiquitous++
		case ears.FormEventDriven:
			dist.EventDriven++
		case ears.FormStateDriven:
			dist.StateDriven++
		case ears.FormOptional:
			dist.Optional++
		case ears.FormUnwanted:
			dist.Unwanted++
		case ears.FormComplex:
			dist.Complex++
		}
	}
	return dist
}

func collectQualitySignals(feature spec.Feature, plan validationPlan, earsResults []EARSCriterion) []string {
	if !plan.validateRequirements || !feature.Requirements.Exists {
		return nil
	}
	var signals []string
	signals = append(signals, signalMissingFailureMode(feature.Requirements.Body, earsResults)...)
	return signals
}

func signalMissingFailureMode(body string, earsResults []EARSCriterion) []string {
	for _, c := range earsResults {
		if c.Form == ears.FormUnwanted {
			return nil
		}
	}
	constraintIDs := sortedKeys(toSet(collectMatches(constraintIDPattern, body)))
	if len(constraintIDs) == 0 {
		return []string{"no unwanted-behavior (IF/THEN) criteria found; consider adding failure mode criteria"}
	}
	return []string{fmt.Sprintf(
		"no unwanted-behavior (IF/THEN) criteria found; consider failure modes for constraints %s",
		strings.Join(constraintIDs, ", "),
	)}
}

func collectEARSResults(feature spec.Feature, plan validationPlan) []EARSCriterion {
	if !plan.validateRequirements || !feature.Requirements.Exists {
		return nil
	}
	parsed := ears.ParseAllCriteria(feature.Requirements.Body)
	results := make([]EARSCriterion, 0, len(parsed))
	for _, c := range parsed {
		results = append(results, EARSCriterion{
			ID:       c.ID,
			Form:     c.Form,
			Valid:    c.Valid,
			Errors:   c.Errors,
			Warnings: c.Warnings,
		})
	}
	return results
}

func collectCoverageReport(feature spec.Feature, plan validationPlan) *CoverageReport {
	if !plan.validateTasks || !feature.Tasks.Exists || !plan.validateRequirements || !feature.Requirements.Exists {
		return nil
	}

	acceptanceCriteria := collectAcceptanceCriteriaIDs(feature.Requirements.Body)

	// Task reference coverage (existing behavior, now reported as a metric).
	taskRefs := extractTaskRequirementRefs(feature.Tasks.Body)
	taskMissing := sortedKeys(setDiff(acceptanceCriteria, taskRefs))

	// Proof reference coverage (new: based on covers: field).
	proofCovers := map[string]struct{}{}
	tree, err := spec.ParseTaskTree(feature.Tasks)
	if err == nil {
		for _, task := range tree.LeafTasks() {
			for _, step := range task.Proof.Steps {
				for _, id := range step.Covers {
					proofCovers[id] = struct{}{}
				}
			}
		}
	}
	proofMissing := sortedKeys(setDiff(acceptanceCriteria, proofCovers))

	return &CoverageReport{
		TaskReferenceCoverage: CoverageStatus{
			Complete: len(taskMissing) == 0,
			Missing:  taskMissing,
		},
		ProofReferenceCoverage: CoverageStatus{
			Complete: len(proofMissing) == 0,
			Missing:  proofMissing,
		},
	}
}

func collectWarnings(feature spec.Feature, plan validationPlan) []string {
	var warnings []string
	if plan.validateTasks && feature.Tasks.Exists {
		warnings = append(warnings, collectLegacyProofWarnings(feature.Tasks)...)
	}
	return warnings
}

func collectLegacyProofWarnings(document spec.Document) []string {
	tree, err := spec.ParseTaskTree(document)
	if err != nil {
		return nil
	}
	var warnings []string
	for _, task := range tree.LeafTasks() {
		if task.Proof.LegacyCommand != "" && len(task.Proof.Steps) == 0 {
			warnings = append(warnings, fmt.Sprintf(
				"task %s uses deprecated legacy verification format; migrate to structured command: [\"...\"] format",
				task.ID,
			))
		}
	}
	return warnings
}

func resolveValidationPlan(feature spec.Feature, scope Scope) (validationPlan, error) {
	switch scope {
	case ScopeFullSpec:
		return validationPlan{
			scope:                scope,
			validateRequirements: feature.Requirements.Exists,
			validateDesign:       feature.Design.Exists,
			validateTasks:        feature.Tasks.Exists,
		}, nil
	case ScopeCurrentPhase:
		state := workflow.ResolveFeatureState(feature)
		switch state.CurrentPhase {
		case workflow.PhaseRequirements:
			return validationPlan{
				scope:                scope,
				validateRequirements: feature.Requirements.Exists,
			}, nil
		case workflow.PhaseDesign:
			return validationPlan{
				scope:                scope,
				validateRequirements: feature.Requirements.Exists,
				validateDesign:       feature.Design.Exists,
			}, nil
		case workflow.PhaseTasks:
			return validationPlan{
				scope:                scope,
				validateRequirements: feature.Requirements.Exists,
				validateDesign:       feature.Design.Exists,
				validateTasks:        feature.Tasks.Exists,
			}, nil
		default:
			return validationPlan{}, fmt.Errorf("resolve validation plan: unsupported phase %q", state.CurrentPhase)
		}
	default:
		return validationPlan{}, fmt.Errorf("resolve validation plan: unsupported scope %q", scope)
	}
}

func (plan validationPlan) validatedPhases() []string {
	phases := make([]string, 0, 3)
	if plan.validateRequirements {
		phases = append(phases, string(workflow.PhaseRequirements))
	}
	if plan.validateDesign {
		phases = append(phases, string(workflow.PhaseDesign))
	}
	if plan.validateTasks {
		phases = append(phases, string(workflow.PhaseTasks))
	}
	return phases
}

func (plan validationPlan) skippedPhases() []string {
	skipped := make([]string, 0, 3)
	if !plan.validateRequirements {
		skipped = append(skipped, string(workflow.PhaseRequirements))
	}
	if !plan.validateDesign {
		skipped = append(skipped, string(workflow.PhaseDesign))
	}
	if !plan.validateTasks {
		skipped = append(skipped, string(workflow.PhaseTasks))
	}
	return skipped
}

func validateDocuments(feature spec.Feature, plan validationPlan) error {
	if plan.validateRequirements && feature.Requirements.Exists {
		if err := validateRequirements(feature.Requirements); err != nil {
			return err
		}
	}
	if plan.validateDesign && feature.Design.Exists {
		if err := validateDesign(feature.Design); err != nil {
			return err
		}
	}
	if plan.validateTasks && feature.Tasks.Exists {
		if err := validateTasks(feature.Tasks); err != nil {
			return err
		}
	}

	if err := validatePrerequisites(feature); err != nil {
		return err
	}

	if err := validateFreshness(feature, plan); err != nil {
		return err
	}
	if err := validateRequirementReferences(feature, plan); err != nil {
		return err
	}

	return nil
}

func validateRequirements(document spec.Document) error {
	if err := expectKeys(document, "status", "approved_at", "last_modified"); err != nil {
		return fmt.Errorf("requirements.md: %w", err)
	}
	if err := validateCommonFields("requirements.md", document); err != nil {
		return err
	}

	requirementIDs := collectSet(requirementHeader.FindAllStringSubmatch(document.Body, -1), 1)
	if len(requirementIDs) == 0 {
		return fmt.Errorf("requirements.md: no requirement IDs found")
	}

	acceptanceIDs := collectMatches(acceptanceIDPattern, document.Body)
	if len(acceptanceIDs) == 0 {
		return fmt.Errorf("requirements.md: no acceptance criteria IDs found")
	}

	for _, acceptanceID := range acceptanceIDs {
		requirementID := strings.Split(acceptanceID, ".AC")[0]
		if _, ok := requirementIDs[requirementID]; !ok {
			return fmt.Errorf(
				"requirements.md: acceptance criterion %q does not map to a requirement",
				acceptanceID,
			)
		}
	}

	parsed := ears.ParseAllCriteria(document.Body)
	for _, criterion := range parsed {
		if !criterion.Valid {
			return fmt.Errorf(
				"requirements.md: acceptance criterion %s has invalid EARS syntax: %s",
				criterion.ID, strings.Join(criterion.Errors, "; "),
			)
		}
	}

	return nil
}

func validateDesign(document spec.Document) error {
	if err := expectKeys(document, "status", "approved_at", "last_modified", "source_requirements_approved_at"); err != nil {
		return fmt.Errorf("design.md: %w", err)
	}
	if err := validateCommonFields("design.md", document); err != nil {
		return err
	}

	missing := []string{}
	for _, section := range requiredSectionsDesign {
		if !strings.Contains(document.Body, section) {
			missing = append(missing, section)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("design.md: missing required sections %s", strings.Join(missing, ", "))
	}

	if document.Status == "approved" {
		if err := validateTimestamp("design.md", "source_requirements_approved_at", document.SourceRequirementsApprovedAt); err != nil {
			return err
		}
	}

	return nil
}

func validateTasks(document spec.Document) error {
	if err := expectKeys(document, "status", "approved_at", "last_modified", "source_design_approved_at"); err != nil {
		return fmt.Errorf("tasks.md: %w", err)
	}
	if err := validateCommonFields("tasks.md", document); err != nil {
		return err
	}

	if !checkboxPattern.MatchString(document.Body) {
		return fmt.Errorf("tasks.md: no checkbox tasks found")
	}

	// For in-review and approved documents, use the full task parser to catch
	// malformed proof definitions at validation time rather than execution time.
	if document.Status == "in-review" || document.Status == "approved" {
		_, err := spec.ParseTaskTree(document)
		if err != nil {
			return fmt.Errorf("tasks.md: %v", err)
		}
	} else {
		// For draft documents, use lightweight string-presence checks to allow
		// incremental authoring without requiring complete proof definitions.
		subtaskBlocks := extractSubtaskBlocks(document.Body)
		if len(subtaskBlocks) == 0 {
			return fmt.Errorf("tasks.md: no leaf tasks found")
		}

		for taskID, lines := range subtaskBlocks {
			blockText := strings.Join(lines, "\n")
			if !strings.Contains(blockText, "Requirements:") {
				return fmt.Errorf("tasks.md: task %s is missing Requirements", taskID)
			}
			if !strings.Contains(blockText, "Design:") {
				return fmt.Errorf("tasks.md: task %s is missing Design", taskID)
			}
			if !strings.Contains(blockText, "Verification:") {
				return fmt.Errorf("tasks.md: task %s is missing Verification", taskID)
			}
		}
	}

	if document.Status == "approved" {
		if err := validateTimestamp("tasks.md", "source_design_approved_at", document.SourceDesignApprovedAt); err != nil {
			return err
		}
	}

	return nil
}

func validateCommonFields(filename string, document spec.Document) error {
	if _, ok := statusValues[document.Status]; !ok {
		return fmt.Errorf("%s: invalid status %q", filename, document.Status)
	}
	if err := validateTimestamp(filename, "last_modified", document.LastModified); err != nil {
		return err
	}
	if document.ApprovedAt != "" {
		if err := validateTimestamp(filename, "approved_at", document.ApprovedAt); err != nil {
			return err
		}
	}
	if document.Status == "approved" && document.ApprovedAt == "" {
		return fmt.Errorf("%s: approved documents must include approved_at", filename)
	}
	return nil
}

func validateTimestamp(filename, field, value string) error {
	if value == "" {
		return fmt.Errorf("%s: %s is required", filename, field)
	}
	if _, err := spec.ParseWaldenTimestamp(value); err != nil {
		return fmt.Errorf("%s: invalid %s %q", filename, field, value)
	}
	return nil
}

func expectKeys(document spec.Document, required ...string) error {
	missing := []string{}
	for _, key := range required {
		if _, ok := document.Fields[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing frontmatter keys %s", strings.Join(missing, ", "))
	}
	return nil
}

func validatePrerequisites(feature spec.Feature) error {
	if feature.Design.Exists && !feature.Requirements.Exists {
		return fmt.Errorf("design.md exists without requirements.md")
	}
	if feature.Tasks.Exists && !feature.Design.Exists {
		return fmt.Errorf("tasks.md exists without design.md")
	}
	if feature.Tasks.Exists && !feature.Requirements.Exists {
		return fmt.Errorf("tasks.md exists without requirements.md")
	}
	if feature.Design.Exists && feature.Requirements.Exists &&
		feature.Design.Status == "approved" && feature.Requirements.Status != "approved" {
		return fmt.Errorf("approved design requires approved requirements")
	}
	if feature.Tasks.Exists && feature.Design.Exists &&
		feature.Tasks.Status == "approved" && feature.Design.Status != "approved" {
		return fmt.Errorf("approved tasks require approved design")
	}
	return nil
}

func validateFreshness(feature spec.Feature, plan validationPlan) error {
	if plan.validateDesign && feature.Design.Exists && feature.Design.Status == "approved" {
		equal, err := timestampsEqual(feature.Requirements.ApprovedAt, feature.Design.SourceRequirementsApprovedAt)
		if err != nil {
			return fmt.Errorf("design.md freshness check: %w", err)
		}
		if !equal {
			return fmt.Errorf("design.md is stale relative to requirements.md")
		}
	}
	if plan.validateTasks && feature.Tasks.Exists && feature.Tasks.Status == "approved" {
		equal, err := timestampsEqual(feature.Design.ApprovedAt, feature.Tasks.SourceDesignApprovedAt)
		if err != nil {
			return fmt.Errorf("tasks.md freshness check: %w", err)
		}
		if !equal {
			return fmt.Errorf("tasks.md is stale relative to design.md")
		}
	}
	return nil
}

func timestampsEqual(a, b string) (bool, error) {
	if a == "" || b == "" {
		return a == b, nil
	}
	return spec.TimestampsEqual(a, b)
}

func validateRequirementReferences(feature spec.Feature, plan validationPlan) error {
	requirementCatalog := collectRequirementCatalog(feature.Requirements.Body)
	functionalRequirements := collectFunctionalRequirementIDs(feature.Requirements.Body)
	nonFunctionalRequirements := collectNFRIDs(feature.Requirements.Body)

	if plan.validateDesign && feature.Design.Body != "" {
		unknown := setDiff(collectBacktickIDs(feature.Design.Body), requirementCatalog)
		if len(unknown) > 0 {
			return fmt.Errorf("design.md references unknown IDs: %s", strings.Join(sortedKeys(unknown), ", "))
		}

		designCoverage := collectCoverageIDs(feature.Design.Body)
		missingDesign := setDiff(union(functionalRequirements, nonFunctionalRequirements), designCoverage)
		if len(missingDesign) > 0 {
			return fmt.Errorf("design.md missing coverage rows for IDs: %s", strings.Join(sortedKeys(missingDesign), ", "))
		}
	}

	if plan.validateTasks && feature.Tasks.Body != "" {
		taskRefs := extractTaskRequirementRefs(feature.Tasks.Body)
		unknown := setDiff(taskRefs, requirementCatalog)
		if len(unknown) > 0 {
			return fmt.Errorf("tasks.md references unknown IDs: %s", strings.Join(sortedKeys(unknown), ", "))
		}
		for ref := range taskRefs {
			if parts := strings.SplitN(ref, ".AC", 2); len(parts) == 2 {
				taskRefs[parts[0]] = struct{}{}
			}
		}
		missingTaskCoverage := setDiff(functionalRequirements, taskRefs)
		if len(missingTaskCoverage) > 0 {
			return fmt.Errorf("tasks.md missing task coverage for requirement IDs: %s", strings.Join(sortedKeys(missingTaskCoverage), ", "))
		}
		acceptanceCriteria := collectAcceptanceCriteriaIDs(feature.Requirements.Body)
		missingACCoverage := setDiff(acceptanceCriteria, taskRefs)
		if len(missingACCoverage) > 0 {
			return fmt.Errorf("tasks.md missing coverage for acceptance criteria: %s", strings.Join(sortedKeys(missingACCoverage), ", "))
		}

		// Validate that covers: references point to known AC IDs.
		tree, err := spec.ParseTaskTree(feature.Tasks)
		if err == nil {
			for _, task := range tree.LeafTasks() {
				for _, step := range task.Proof.Steps {
					for _, coverID := range step.Covers {
						if _, ok := requirementCatalog[coverID]; !ok {
							return fmt.Errorf("tasks.md: task %s covers unknown ID %q", task.ID, coverID)
						}
					}
				}
			}
		}
	}

	return nil
}

func extractSubtaskBlocks(body string) map[string][]string {
	blocks := map[string][]string{}
	lines := strings.Split(body, "\n")
	currentID := ""
	currentLines := []string{}

	flush := func() {
		if currentID != "" {
			cloned := make([]string, len(currentLines))
			copy(cloned, currentLines)
			blocks[currentID] = cloned
		}
		currentID = ""
		currentLines = nil
	}

	for _, line := range lines {
		if match := subtaskPattern.FindStringSubmatch(line); match != nil {
			flush()
			taskID := match[2]
			if strings.Contains(taskID, ".") {
				currentID = taskID
				currentLines = []string{}
			}
			continue
		}

		if currentID != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	return blocks
}

func collectRequirementCatalog(body string) map[string]struct{} {
	catalog := collectSet(requirementHeader.FindAllStringSubmatch(body, -1), 1)
	mergeInto(catalog, toSet(collectMatches(acceptanceIDPattern, body)))
	mergeInto(catalog, toSet(collectMatches(nfrIDPattern, body)))
	mergeInto(catalog, toSet(collectMatches(constraintIDPattern, body)))
	return catalog
}

func collectFunctionalRequirementIDs(body string) map[string]struct{} {
	return collectSet(requirementHeader.FindAllStringSubmatch(body, -1), 1)
}

func collectAcceptanceCriteriaIDs(body string) map[string]struct{} {
	return toSet(collectMatches(acceptanceIDPattern, body))
}

func collectNFRIDs(body string) map[string]struct{} {
	return toSet(collectMatches(nfrIDPattern, body))
}

func collectBacktickIDs(body string) map[string]struct{} {
	return toSet(collectMatches(backtickIDPattern, body))
}

func collectCoverageIDs(body string) map[string]struct{} {
	return collectSet(coverageRowPattern.FindAllStringSubmatch(body, -1), 1)
}

func extractTaskRequirementRefs(body string) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "Requirements:") {
			mergeInto(refs, toSet(collectMatches(backtickIDPattern, line)))
		}
	}
	return refs
}

func collectMatches(pattern *regexp.Regexp, body string) []string {
	matches := pattern.FindAllStringSubmatch(body, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	return values
}

func collectSet(matches [][]string, index int) map[string]struct{} {
	values := map[string]struct{}{}
	for _, match := range matches {
		if len(match) > index {
			values[match[index]] = struct{}{}
		}
	}
	return values
}

func toSet(values []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func union(left, right map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	mergeInto(out, left)
	mergeInto(out, right)
	return out
}

func mergeInto(target, source map[string]struct{}) {
	for key := range source {
		target[key] = struct{}{}
	}
}

func setDiff(left, right map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for key := range left {
		if _, ok := right[key]; !ok {
			out[key] = struct{}{}
		}
	}
	return out
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
