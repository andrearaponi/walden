package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/andrearaponi/walden/internal/evidence"
)

// Result is the shared structured output model for CLI commands.
type Result struct {
	Summary               string              `json:"summary"`
	CreatedFiles          []string            `json:"created_files,omitempty"`
	UpdatedFiles          []string            `json:"updated_files,omitempty"`
	ChangedFiles          []string            `json:"changed_files"`
	SkippedFiles          []string            `json:"skipped_files"`
	CompletedTasks        []string            `json:"completed_tasks,omitempty"`
	AutoCompleted         []string            `json:"auto_completed,omitempty"`
	ValidatedPhases       []string            `json:"validated_phases,omitempty"`
	SkippedPhases         []string            `json:"skipped_phases,omitempty"`
	GitInitialized        bool                `json:"git_initialized,omitempty"`
	GitAlreadyInitialized bool                `json:"git_already_initialized,omitempty"`
	CurrentPhase          string              `json:"current_phase,omitempty"`
	DocumentSchemaVersion string              `json:"document_schema_version,omitempty"`
	Completion            string              `json:"completion,omitempty"`
	CertifiedCommit       string              `json:"certified_commit,omitempty"`
	BranchName            string              `json:"branch_name,omitempty"`
	Document              string              `json:"document,omitempty"`
	Documents             []DocumentStatus    `json:"documents,omitempty"`
	Task                  *TaskStatus         `json:"task,omitempty"`
	Blockers              []string            `json:"blockers,omitempty"`
	NextAction            string              `json:"next_action,omitempty"`
	Warnings              []string            `json:"warnings"`
	EARSValidation        []EARSCriterion     `json:"ears_validation,omitempty"`
	Coverage              *CoverageReport     `json:"coverage,omitempty"`
	EARSDistribution      *EARSDistribution   `json:"ears_distribution,omitempty"`
	Features              []FeatureValidation `json:"features,omitempty"`
	Evidence              []EvidenceStatus    `json:"evidence,omitempty"`
	Scope                 *evidence.Scope     `json:"scope,omitempty"`
	Update                *UpdateStatus       `json:"update,omitempty"`
	Release               *ReleaseStatus      `json:"release,omitempty"`
	Adoption              *AdoptionStatus     `json:"adoption,omitempty"`
	Waiver                *WaiverStatus       `json:"waiver,omitempty"`
	Content               string              `json:"content,omitempty"`
	ExitCode              int                 `json:"exit_code"`
}

// FeatureValidation is one feature's verdict in a portfolio validation run.
type FeatureValidation struct {
	Feature string `json:"feature"`
	Valid   bool   `json:"valid"`
	Summary string `json:"summary"`
}

// ReleaseStatus is the JSON output view of a release certification run.
type ReleaseStatus struct {
	Scope      *evidence.Scope  `json:"scope,omitempty"`
	Releasable bool             `json:"releasable"`
	Strict     bool             `json:"strict"`
	Features   []ReleaseFeature `json:"features"`
	Worktree   ReleaseWorktree  `json:"worktree"`
}

// ReleaseFeature is one feature's per-criterion certification outcome.
type ReleaseFeature struct {
	Feature  string             `json:"feature"`
	Criteria []ReleaseCriterion `json:"criteria"`
	Pending  []string           `json:"pending,omitempty"`
	Evidence []EvidenceStatus   `json:"evidence,omitempty"`
}

// ReleaseCriterion is a single certification criterion's verdict.
type ReleaseCriterion struct {
	Name     string   `json:"name"`
	Passed   bool     `json:"passed"`
	Blockers []string `json:"blockers,omitempty"`
}

// ReleaseWorktree is the repository-level worktree criterion's outcome.
type ReleaseWorktree struct {
	Blockers     []string       `json:"blockers,omitempty"`
	WaldenDirty  []string       `json:"walden_dirty,omitempty"`
	GitSkipped   bool           `json:"git_skipped"`
	InputBinding string         `json:"input_binding,omitempty"`
	Inputs       []ReleaseInput `json:"inputs,omitempty"`
}

type ReleaseInput struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// WaiverStatus is the verdict-carried record of a pending-task waiver: the
// operator's reason and the feature-qualified tasks it covered.
type WaiverStatus struct {
	Reason string   `json:"reason"`
	Tasks  []string `json:"tasks"`
}

// AdoptionStatus is the JSON output view of a brownfield adoption plan or run.
type AdoptionStatus struct {
	Scope    *evidence.Scope   `json:"scope,omitempty"`
	Apply    bool              `json:"apply"`
	Features []AdoptionFeature `json:"features"`
}

// AdoptionFeature is one feature's adoption classification and outcome.
type AdoptionFeature struct {
	Feature      string           `json:"feature"`
	Class        string           `json:"class"`
	SealableDocs []string         `json:"sealable_docs,omitempty"`
	SealedDocs   []string         `json:"sealed_docs,omitempty"`
	ReproveCount int              `json:"reprove_count,omitempty"`
	Evidence     []EvidenceStatus `json:"evidence,omitempty"`
	Verified     []string         `json:"verified,omitempty"`
	Failed       []string         `json:"failed,omitempty"`
	Skipped      int              `json:"skipped,omitempty"`
	Reason       string           `json:"reason,omitempty"`
}

// EvidenceStatus is the JSON output view of one task's execution evidence.
type EvidenceStatus struct {
	TaskID           string                        `json:"task_id"`
	State            string                        `json:"state"`
	Passed           *bool                         `json:"passed,omitempty"`
	Failure          string                        `json:"failure,omitempty"`
	RecordedIdentity string                        `json:"recorded_identity,omitempty"`
	CurrentIdentity  string                        `json:"current_identity,omitempty"`
	Profile          map[string]string             `json:"profile,omitempty"`
	ProfileDrift     []ProfileDriftEntry           `json:"profile_drift,omitempty"`
	ProfileLegacy    bool                          `json:"profile_legacy,omitempty"`
	Binding          *evidence.BindingAssessment   `json:"binding,omitempty"`
	CodeFreshness    string                        `json:"code_freshness,omitempty"`
	Execution        *evidence.ExecutionAssessment `json:"execution,omitempty"`
	Gaps             []evidence.Gap                `json:"gaps,omitempty"`
}

// ProfileDriftEntry is one differing execution-profile entry: the recorded
// value against the current machine's.
type ProfileDriftEntry struct {
	Key      string `json:"key"`
	Recorded string `json:"recorded"`
	Current  string `json:"current"`
}

// UpdateStatus is the JSON output view of an update check or run.
type UpdateStatus struct {
	CurrentVersion  string `json:"current_version"`
	TargetVersion   string `json:"target_version"`
	UpdateAvailable bool   `json:"update_available"`
	Applied         bool   `json:"applied"`
}

// EARSDistribution is the JSON output view of EARS form counts.
type EARSDistribution struct {
	Ubiquitous  int `json:"ubiquitous"`
	EventDriven int `json:"event_driven"`
	StateDriven int `json:"state_driven"`
	Optional    int `json:"optional"`
	Unwanted    int `json:"unwanted"`
	Complex     int `json:"complex"`
	Total       int `json:"total"`
}

// CoverageReport is the JSON output view of task and proof coverage metrics.
type CoverageReport struct {
	TaskReferenceCoverage  CoverageStatus `json:"task_reference_coverage"`
	ProofReferenceCoverage CoverageStatus `json:"proof_reference_coverage"`
}

// CoverageStatus reports whether coverage is complete and which IDs are missing.
type CoverageStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing,omitempty"`
}

// EARSCriterion is the JSON output view of an EARS parse result.
type EARSCriterion struct {
	ID       string   `json:"id"`
	Form     string   `json:"form"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// DocumentStatus is the shared output view for one Walden document.
type DocumentStatus struct {
	Name                string   `json:"name"`
	Status              string   `json:"status"`
	Fresh               bool     `json:"fresh"`
	ApprovedAt          string   `json:"approved_at,omitempty"`
	ApprovedFingerprint string   `json:"approved_fingerprint,omitempty"`
	StaleCauses         []string `json:"stale_causes,omitempty"`
}

// TaskStatus is the shared output view for one execution task context.
type TaskStatus struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	ParentID     string   `json:"parent_id,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	DesignRefs   []string `json:"design_refs,omitempty"`
	Verification string   `json:"verification,omitempty"`
}

// PrintText renders a compact human-readable result summary.
func PrintText(w io.Writer, result Result) {
	_, _ = fmt.Fprintf(w, "Summary: %s\n", result.Summary)
	if result.Scope != nil {
		_, _ = fmt.Fprintf(w, "Scope: %s; guarantee: %s\n", result.Scope.Description(), result.Scope.Guarantee)
	}

	if len(result.CreatedFiles) > 0 {
		_, _ = fmt.Fprintln(w, "Created files:")
		for _, path := range result.CreatedFiles {
			_, _ = fmt.Fprintf(w, "- %s\n", path)
		}
	}

	if len(result.UpdatedFiles) > 0 {
		_, _ = fmt.Fprintln(w, "Updated files:")
		for _, path := range result.UpdatedFiles {
			_, _ = fmt.Fprintf(w, "- %s\n", path)
		}
	}

	if len(result.CreatedFiles) == 0 && len(result.UpdatedFiles) == 0 && len(result.ChangedFiles) > 0 {
		_, _ = fmt.Fprintln(w, "Changed files:")
		for _, path := range result.ChangedFiles {
			_, _ = fmt.Fprintf(w, "- %s\n", path)
		}
	}

	if len(result.SkippedFiles) > 0 {
		_, _ = fmt.Fprintln(w, "Skipped files:")
		for _, path := range result.SkippedFiles {
			_, _ = fmt.Fprintf(w, "- %s\n", path)
		}
	}

	if len(result.CompletedTasks) > 0 {
		_, _ = fmt.Fprintf(w, "Completed tasks: %s\n", strings.Join(result.CompletedTasks, ", "))
	}

	if len(result.AutoCompleted) > 0 {
		_, _ = fmt.Fprintf(w, "Auto-completed tasks: %s\n", strings.Join(result.AutoCompleted, ", "))
	}

	if result.CurrentPhase != "" {
		_, _ = fmt.Fprintf(w, "Current phase: %s\n", result.CurrentPhase)
	}

	switch {
	case result.GitInitialized:
		_, _ = fmt.Fprintln(w, "Git: initialized new repository")
	case result.GitAlreadyInitialized:
		_, _ = fmt.Fprintln(w, "Git: repository already initialized")
	}

	if len(result.ValidatedPhases) > 0 {
		_, _ = fmt.Fprintf(w, "Validated phases: %s\n", strings.Join(result.ValidatedPhases, ", "))
	}

	if len(result.SkippedPhases) > 0 {
		_, _ = fmt.Fprintf(w, "Skipped phases: %s\n", strings.Join(result.SkippedPhases, ", "))
	}

	if result.BranchName != "" {
		_, _ = fmt.Fprintf(w, "Branch: %s\n", result.BranchName)
	}

	if result.Document != "" {
		_, _ = fmt.Fprintf(w, "Document: %s\n", result.Document)
	}

	if len(result.Documents) > 0 {
		_, _ = fmt.Fprintln(w, "Documents:")
		for _, document := range result.Documents {
			_, _ = fmt.Fprintf(w, "- %s: status=%s fresh=%t", document.Name, document.Status, document.Fresh)
			if document.ApprovedAt != "" {
				_, _ = fmt.Fprintf(w, " approved_at=%s", document.ApprovedAt)
			}
			if len(document.StaleCauses) > 0 {
				_, _ = fmt.Fprintf(w, " causes: %s", strings.Join(document.StaleCauses, "; "))
			}
			_, _ = fmt.Fprintln(w)
		}
	}

	if result.Task != nil {
		_, _ = fmt.Fprintf(w, "Task: %s %s\n", result.Task.ID, result.Task.Title)
		if result.Task.ParentID != "" {
			_, _ = fmt.Fprintf(w, "Task parent: %s\n", result.Task.ParentID)
		}
		if len(result.Task.Requirements) > 0 {
			_, _ = fmt.Fprintf(w, "Task requirements: %s\n", strings.Join(result.Task.Requirements, ", "))
		}
		if len(result.Task.DesignRefs) > 0 {
			_, _ = fmt.Fprintf(w, "Task design refs: %s\n", strings.Join(result.Task.DesignRefs, ", "))
		}
		if result.Task.Verification != "" {
			_, _ = fmt.Fprintf(w, "Task verification: %s\n", result.Task.Verification)
		}
	}

	if len(result.Features) > 0 {
		_, _ = fmt.Fprintln(w, "Features:")
		for _, feature := range result.Features {
			verdict := "VALID"
			if !feature.Valid {
				verdict = feature.Summary
			}
			_, _ = fmt.Fprintf(w, "- %s: %s\n", feature.Feature, verdict)
		}
	}

	if len(result.Evidence) > 0 {
		_, _ = fmt.Fprintln(w, "Evidence:")
		printEvidence(w, result.Evidence, "")
	}

	if result.Update != nil {
		_, _ = fmt.Fprintf(w, "Update: current=%s target=%s available=%t applied=%t\n",
			result.Update.CurrentVersion, result.Update.TargetVersion, result.Update.UpdateAvailable, result.Update.Applied)
	}

	if result.Adoption != nil {
		_, _ = fmt.Fprintln(w, "Adoption:")
		if result.Adoption.Scope != nil {
			_, _ = fmt.Fprintf(w, "Scope: %s; guarantee: %s\n", result.Adoption.Scope.Description(), result.Adoption.Scope.Guarantee)
		}
		for _, feature := range result.Adoption.Features {
			_, _ = fmt.Fprintf(w, "- %s: %s", feature.Feature, feature.Class)
			if len(feature.SealableDocs) > 0 {
				_, _ = fmt.Fprintf(w, " (%d doc(s) to seal, %d task(s) to re-prove)", len(feature.SealableDocs), feature.ReproveCount)
			} else if !result.Adoption.Apply && feature.ReproveCount > 0 {
				_, _ = fmt.Fprintf(w, " (%d task(s) to re-prove)", feature.ReproveCount)
			}
			if len(feature.SealedDocs) > 0 {
				_, _ = fmt.Fprintf(w, " sealed=%d", len(feature.SealedDocs))
			}
			if len(feature.Verified) > 0 || len(feature.Failed) > 0 || feature.Skipped > 0 {
				_, _ = fmt.Fprintf(w, " verified=%d failed=%d skipped=%d", len(feature.Verified), len(feature.Failed), feature.Skipped)
			}
			if feature.Reason != "" {
				_, _ = fmt.Fprintf(w, " (%s)", feature.Reason)
			}
			_, _ = fmt.Fprintln(w)
			printEvidence(w, feature.Evidence, "  ")
		}
	}

	if result.Release != nil {
		_, _ = fmt.Fprintln(w, "Release:")
		if result.Release.Scope != nil {
			_, _ = fmt.Fprintf(w, "Scope: %s; guarantee: %s\n", result.Release.Scope.Description(), result.Release.Scope.Guarantee)
		}
		if result.Release.Worktree.InputBinding != "" {
			_, _ = fmt.Fprintf(w, "- committed input binding: %s\n", result.Release.Worktree.InputBinding)
		}
		for _, feature := range result.Release.Features {
			_, _ = fmt.Fprintf(w, "- %s:", feature.Feature)
			for _, criterion := range feature.Criteria {
				verdict := "pass"
				if !criterion.Passed {
					verdict = "FAIL"
				}
				_, _ = fmt.Fprintf(w, " %s=%s", criterion.Name, verdict)
			}
			if len(feature.Pending) > 0 {
				_, _ = fmt.Fprintf(w, " pending: %s", strings.Join(feature.Pending, ", "))
			}
			_, _ = fmt.Fprintln(w)
			printEvidence(w, feature.Evidence, "  ")
		}
		switch {
		case result.Release.Worktree.GitSkipped:
			_, _ = fmt.Fprintln(w, "- worktree: no usable git (blocking)")
		case len(result.Release.Worktree.Blockers) > 0:
			_, _ = fmt.Fprintf(w, "- worktree: %d uncommitted path(s)\n", len(result.Release.Worktree.Blockers))
		default:
			_, _ = fmt.Fprintln(w, "- worktree: clean")
		}
	}

	if len(result.Blockers) > 0 {
		_, _ = fmt.Fprintln(w, "Blockers:")
		for _, blocker := range result.Blockers {
			_, _ = fmt.Fprintf(w, "- %s\n", blocker)
		}
	}

	if result.NextAction != "" {
		_, _ = fmt.Fprintf(w, "Next action: %s\n", result.NextAction)
	}

	if len(result.Warnings) > 0 {
		_, _ = fmt.Fprintln(w, "Warnings:")
		for _, warning := range result.Warnings {
			_, _ = fmt.Fprintf(w, "- %s\n", warning)
		}
	}

	_, _ = fmt.Fprintf(w, "Exit code: %d\n", result.ExitCode)
}

// Envelope is the versioned JSON wrapper for machine-readable CLI output.
type Envelope struct {
	SchemaVersion string `json:"schema_version"`
	Command       string `json:"command"`
	OK            bool   `json:"ok"`
	Result        Result `json:"result"`
}

// PrintJSON renders the result inside a versioned JSON envelope.
func PrintJSON(w io.Writer, command string, result Result) error {
	envelope := Envelope{
		SchemaVersion: "v0beta1",
		Command:       command,
		OK:            result.ExitCode == 0,
		Result:        result,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(envelope)
}
