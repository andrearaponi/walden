package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andrearaponi/walden/internal/consolidate"
	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/spec"
)

const viewScaffold = `# Current Contracts

<!-- One section per consolidated feature, for example:

## feature-name

Purpose: one line on what the feature guarantees today.
Active: R1.AC1, R1.AC2, NFR1, C1
Reserved: R1.AC3 (removed by decision D4)
Sources: docs/decisions/D1-example.md
Related: other-feature (why they interact)

walden consolidate lists the identifiers of the features in scope.

While changes are pending, record the outcome of comparing each one before walden consolidate open, for example:

## Coherence Review

### feature-name

- No inconsistency: the changed R1.AC1 agrees with ` + "`other-feature#R1.AC2`" + `. -->
`

func runConsolidate(args []string, stdout io.Writer, stderr io.Writer) int {
	if groupHelp("consolidate", args, stdout) {
		return 0
	}
	action, rest := "report", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action, rest = args[0], args[1:]
	}
	switch action {
	case "report":
		return runConsolidateReport(rest, stdout, stderr)
	case "start":
		return runConsolidateStart(rest, stdout, stderr)
	case "open":
		return runConsolidateOpen(rest, stdout, stderr)
	case "approve":
		return runConsolidateApprove(rest, stdout, stderr)
	}
	_, _ = fmt.Fprintf(stderr, "unknown command: consolidate %s\n\n", strings.Join(args, " "))
	printUsage(stderr)
	return 1
}

func consolidationRoot(command, path string, args []string, stdout, stderr io.Writer) (string, bool, int) {
	parsed, handled, code := triageArgs(path, command, args, stdout, stderr)
	if handled {
		return "", true, code
	}
	jsonMode := parsed.Bool("--json")
	if len(parsed.Positionals) > 0 {
		return "", true, emitResult(command, errorResult(fmt.Errorf("walden %s takes no arguments", path)), jsonMode, stdout, stderr)
	}
	root, err := os.Getwd()
	if err != nil {
		return "", true, emitResult(command, errorResult(fmt.Errorf("resolve working directory: %w", err)), jsonMode, stdout, stderr)
	}
	return root, false, 0
}

func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

func runConsolidateReport(args []string, stdout io.Writer, stderr io.Writer) int {
	root, handled, code := consolidationRoot("consolidate", "consolidate report", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := jsonRequested(args)
	inputs, err := consolidate.LoadInputs(root)
	if err != nil {
		return emitResult("consolidate", errorResult(err), jsonMode, stdout, stderr)
	}
	report := inputs.Report(root)
	result := output.Result{
		Summary:       consolidationSummary(report.State),
		Consolidation: consolidationReportView(report),
		Warnings:      consolidationWarnings(report.State),
		NextAction:    consolidationNextAction(report.State),
	}
	return emitResult("consolidate", result, jsonMode, stdout, stderr)
}

func runConsolidateStart(args []string, stdout io.Writer, stderr io.Writer) int {
	root, handled, code := consolidationRoot("consolidate-start", "consolidate start", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := jsonRequested(args)
	fail := func(err error, next string) int {
		result := errorResult(err)
		result.NextAction = next
		return emitResult("consolidate-start", result, jsonMode, stdout, stderr)
	}
	if info, err := os.Stat(filepath.Join(root, ".walden")); err != nil || !info.IsDir() {
		return fail(errors.New("not a Walden repository: .walden/ is missing"), "walden repo init")
	}
	record, err := consolidate.LoadRecord(root)
	if err != nil {
		return fail(err, "restore "+consolidate.RecordPath+" from version control")
	}
	if record != nil {
		return fail(fmt.Errorf("consolidation tracking already started on %s", record.StartedAt), "walden consolidate")
	}
	inputs, err := consolidate.LoadInputs(root)
	if err != nil {
		return fail(err, "")
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	started := consolidate.Record{Schema: consolidate.RecordSchema, StartedAt: now, Features: map[string]consolidate.RecordFeature{}}
	for _, feature := range inputs.Snapshot.Features {
		if feature.Approved() {
			started.Features[feature.Name] = consolidate.RecordFeature{State: consolidate.StateBacklog,
				RequirementsFingerprint: feature.ApprovedFingerprint, ActiveIDs: feature.Definitions.Active(), Statements: feature.ActiveStatements()}
		}
	}

	created := []string{}
	if !inputs.View.Exists {
		if inputs.ViewErr != nil {
			return fail(inputs.ViewErr, "fix the frontmatter of "+consolidate.ViewPath+" or restore it from version control")
		}
		scaffold := spec.Document{Path: filepath.Join(root, consolidate.ViewPath), Body: viewScaffold, Fields: map[string]string{
			"status": "draft", "approved_at": "", "last_modified": now, "approved_fingerprint": ""}}
		if err := spec.SaveDocument(scaffold); err != nil {
			return fail(err, "")
		}
		created = append(created, consolidate.ViewPath)
	}
	if err := consolidate.SaveRecord(root, started); err != nil {
		return fail(err, "")
	}
	created = append([]string{consolidate.RecordPath}, created...)
	result := output.Result{
		Summary:      fmt.Sprintf("consolidation tracking started: %d approved feature(s) recorded as backlog", len(started.Features)),
		CreatedFiles: created,
		ChangedFiles: created,
		NextAction:   "write sections of " + consolidate.ViewPath + " for the features you consolidate, then walden consolidate open",
	}
	return emitResult("consolidate-start", result, jsonMode, stdout, stderr)
}

func viewPreconditions(root string) (consolidate.Inputs, error, string) {
	inputs, err := consolidate.LoadInputs(root)
	switch {
	case err != nil:
		return inputs, err, ""
	case inputs.RecordErr != nil:
		return inputs, inputs.RecordErr, "restore " + consolidate.RecordPath + " from version control"
	case inputs.Record == nil:
		return inputs, errors.New("consolidation tracking has not started"), "walden consolidate start"
	case inputs.ViewErr != nil:
		return inputs, inputs.ViewErr, "fix the frontmatter of " + consolidate.ViewPath + " or restore it from version control"
	case !inputs.View.Exists:
		return inputs, fmt.Errorf("the current-contract view %s is missing", consolidate.ViewPath), "restore " + consolidate.ViewPath + " from version control"
	}
	return inputs, nil, ""
}

// coherenceFindings returns what the coherence review of the view still lacks.
func coherenceFindings(root string, inputs consolidate.Inputs) []consolidate.Finding {
	findings := []consolidate.Finding{}
	for _, finding := range inputs.Report(root).Findings {
		if finding.Kind == consolidate.FindingCoherenceReview {
			findings = append(findings, finding)
		}
	}
	return findings
}

func coherenceResult(findings []consolidate.Finding) output.Result {
	result := output.Result{Summary: fmt.Sprintf("the coherence review in %s is incomplete: %d problem(s)", consolidate.ViewPath, len(findings)), ExitCode: 1,
		NextAction: "record one entry per pending feature under " + consolidate.CoherenceHeading + " in " + consolidate.ViewPath +
			" with the inconsistencies found, or none, citing the linked statements you compared as `feature#ID`, then run the command again"}
	for _, finding := range findings {
		result.Blockers = append(result.Blockers, finding.Message)
	}
	return result
}

func mismatchResult(findings []consolidate.Finding, summary string) output.Result {
	result := output.Result{Summary: summary, ExitCode: 1, NextAction: "fix " + consolidate.ViewPath + " and run the command again"}
	for _, finding := range findings {
		result.Blockers = append(result.Blockers, finding.Message)
	}
	return result
}

func runConsolidateOpen(args []string, stdout io.Writer, stderr io.Writer) int {
	root, handled, code := consolidationRoot("consolidate-open", "consolidate open", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := jsonRequested(args)
	inputs, err, next := viewPreconditions(root)
	if err != nil {
		result := errorResult(err)
		result.NextAction = next
		return emitResult("consolidate-open", result, jsonMode, stdout, stderr)
	}
	if state := inputs.View.Status().State; state == consolidate.ViewApproved {
		result := errorResult(errors.New("the current-contract view is approved and unchanged; nothing to open"))
		result.NextAction = "edit " + consolidate.ViewPath + " to consolidate further changes"
		return emitResult("consolidate-open", result, jsonMode, stdout, stderr)
	}
	if findings := consolidate.CheckView(inputs.View, inputs.Snapshot, inputs.Record); len(findings) > 0 {
		return emitResult("consolidate-open", mismatchResult(findings, fmt.Sprintf("the current-contract view has %d mismatch(es) with the specifications", len(findings))), jsonMode, stdout, stderr)
	}
	if findings := coherenceFindings(root, inputs); len(findings) > 0 {
		return emitResult("consolidate-open", coherenceResult(findings), jsonMode, stdout, stderr)
	}
	document := inputs.View.Document
	document.Fields["status"] = "in-review"
	document.Fields["last_modified"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if err := spec.SaveDocument(document); err != nil {
		return emitResult("consolidate-open", errorResult(err), jsonMode, stdout, stderr)
	}
	result := output.Result{
		Summary:      "current-contract view opened for review",
		UpdatedFiles: []string{consolidate.ViewPath},
		ChangedFiles: []string{consolidate.ViewPath},
		NextAction:   "present " + consolidate.ViewPath + " to the user; after explicit approval run walden consolidate approve",
	}
	return emitResult("consolidate-open", result, jsonMode, stdout, stderr)
}

func runConsolidateApprove(args []string, stdout io.Writer, stderr io.Writer) int {
	root, handled, code := consolidationRoot("consolidate-approve", "consolidate approve", args, stdout, stderr)
	if handled {
		return code
	}
	jsonMode := jsonRequested(args)
	inputs, err, next := viewPreconditions(root)
	if err != nil {
		result := errorResult(err)
		result.NextAction = next
		return emitResult("consolidate-approve", result, jsonMode, stdout, stderr)
	}
	record := inputs.Record
	status := inputs.View.Status()
	repair := status.State == consolidate.ViewApproved && (record.Checkpoint == nil || record.Checkpoint.ViewFingerprint != status.ApprovedFingerprint)
	if status.State != consolidate.ViewInReview && !repair {
		result := errorResult(fmt.Errorf("the current-contract view is %s, not in review", status.State))
		result.NextAction = "walden consolidate open"
		if status.State == consolidate.ViewApproved {
			result = errorResult(errors.New("the current-contract view is already approved and recorded"))
			result.NextAction = ""
		}
		return emitResult("consolidate-approve", result, jsonMode, stdout, stderr)
	}
	if findings := consolidate.CheckView(inputs.View, inputs.Snapshot, record); len(findings) > 0 {
		return emitResult("consolidate-approve", mismatchResult(findings, fmt.Sprintf("the current-contract view has %d mismatch(es) with the specifications", len(findings))), jsonMode, stdout, stderr)
	}
	// A repair completes an approval the user already gave; it re-checks nothing new.
	if findings := coherenceFindings(root, inputs); !repair && len(findings) > 0 {
		return emitResult("consolidate-approve", coherenceResult(findings), jsonMode, stdout, stderr)
	}
	unapproved := []string{}
	for _, section := range inputs.View.Sections {
		if feature, _ := inputs.Snapshot.Feature(section.Feature); !feature.Approved() {
			unapproved = append(unapproved, fmt.Sprintf("%s lacks approved and fresh requirements — approve its revision first", section.Feature))
		}
	}
	if len(unapproved) > 0 {
		result := output.Result{Summary: fmt.Sprintf("%d covered feature(s) lack approved and fresh requirements", len(unapproved)), Blockers: unapproved, ExitCode: 1,
			NextAction: "complete the pending requirements reviews, then walden consolidate approve"}
		return emitResult("consolidate-approve", result, jsonMode, stdout, stderr)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	updated := []string{}
	fingerprint := status.ApprovedFingerprint
	if !repair {
		document := inputs.View.Document
		fingerprint = spec.Fingerprint(document.Path, document.Body)
		document.Fields["status"] = "approved"
		document.Fields["approved_at"] = now
		document.Fields["last_modified"] = now
		document.Fields["approved_fingerprint"] = fingerprint
		if err := spec.SaveDocument(document); err != nil {
			return emitResult("consolidate-approve", errorResult(err), jsonMode, stdout, stderr)
		}
		updated = append(updated, consolidate.ViewPath)
	}
	for _, section := range inputs.View.Sections {
		feature, _ := inputs.Snapshot.Feature(section.Feature)
		record.Features[section.Feature] = consolidate.RecordFeature{State: consolidate.StateConsolidated,
			RequirementsFingerprint: feature.ApprovedFingerprint, ActiveIDs: feature.Definitions.Active(), ReservedIDs: unique(section.Reserved),
			Statements: feature.ActiveStatements()}
	}
	for name := range record.Features {
		if _, exists := inputs.Snapshot.Feature(name); !exists {
			delete(record.Features, name)
		}
	}
	record.Checkpoint = &consolidate.Checkpoint{At: now, ViewFingerprint: fingerprint}
	if err := consolidate.SaveRecord(root, *record); err != nil {
		return emitResult("consolidate-approve", errorResult(err), jsonMode, stdout, stderr)
	}
	updated = append(updated, consolidate.RecordPath)

	state := consolidate.Derive(inputs.Snapshot, record, nil, consolidate.ViewStatus{State: consolidate.ViewApproved, ApprovedFingerprint: fingerprint})
	summary := fmt.Sprintf("consolidation approved: %d feature(s) consolidated; %d pending change(s) remain", len(inputs.View.Sections), len(state.Pending))
	if repair {
		summary = "consolidation record repaired from the approved view: " + summary
	}
	result := output.Result{Summary: summary, UpdatedFiles: updated, ChangedFiles: updated, Consolidation: consolidationStateView(state)}
	return emitResult("consolidate-approve", result, jsonMode, stdout, stderr)
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func consolidationStateView(state consolidate.State) *output.ConsolidationStatus {
	view := &output.ConsolidationStatus{Tracking: state.Tracking, Pending: []output.ConsolidationChange{}, Backlog: []string{},
		Unconsolidated: state.Unconsolidated, Threshold: state.Threshold, View: state.View.State, Problem: state.Problem, Remedy: state.Remedy}
	for _, change := range state.Pending {
		view.Pending = append(view.Pending, output.ConsolidationChange{Feature: change.Feature, Kind: change.Kind})
	}
	view.Backlog = append(view.Backlog, state.Backlog...)
	if view.View == "" {
		view.View = consolidate.ViewAbsent
	}
	return view
}

func consolidationReportView(report consolidate.Report) *output.ConsolidationStatus {
	view := consolidationStateView(report.State)
	for _, entry := range report.Scope {
		scoped := output.ConsolidationScopeEntry{Feature: entry.Feature, Pending: entry.Pending, HubOnly: entry.HubOnly}
		for _, link := range entry.Links {
			scoped.Links = append(scoped.Links, output.ConsolidationLink{Kind: link.Kind, Feature: link.Feature, Path: link.Path})
		}
		view.Scope = append(view.Scope, scoped)
	}
	for _, hub := range report.Hubs {
		view.Hubs = append(view.Hubs, output.ConsolidationHub{Path: hub.Path, Features: hub.Features})
	}
	for _, finding := range report.Findings {
		view.Findings = append(view.Findings, output.ConsolidationFinding{Kind: finding.Kind, Feature: finding.Feature, ID: finding.ID, Subject: finding.Subject, Message: finding.Message})
	}
	if len(report.Identifiers) > 0 {
		view.Identifiers = report.Identifiers
	}
	statements := func(items []consolidate.ComparedStatement) []output.ConsolidationStatement {
		out := []output.ConsolidationStatement{}
		for _, item := range items {
			out = append(out, output.ConsolidationStatement{ID: item.ID, Text: item.Text, Mark: item.Mark, Previous: item.Previous})
		}
		return out
	}
	for _, comparison := range report.Comparisons {
		entry := output.ConsolidationComparison{Feature: comparison.Feature, Kind: comparison.Kind, Statements: statements(comparison.Statements)}
		for _, linked := range comparison.Linked {
			item := output.ConsolidationLinked{Feature: linked.Feature, HubOnly: linked.HubOnly}
			for _, link := range linked.Links {
				item.Links = append(item.Links, output.ConsolidationLink{Kind: link.Kind, Feature: link.Feature, Path: link.Path})
			}
			if len(linked.Statements) > 0 {
				item.Statements = statements(linked.Statements)
			}
			entry.Linked = append(entry.Linked, item)
		}
		view.Comparisons = append(view.Comparisons, entry)
	}
	return view
}

func pendingList(state consolidate.State) string {
	names := []string{}
	for _, change := range state.Pending {
		names = append(names, change.Feature)
	}
	return strings.Join(names, ", ")
}

// attachConsolidation adds the read-only consolidation state to a status
// result: an additive JSON field, a brief text note and shared warnings. A
// consolidation failure never changes the host command's outcome.
func attachConsolidation(root string, result *output.Result) {
	inputs, err := consolidate.LoadInputs(root)
	if err != nil {
		return
	}
	state := inputs.State()
	view := consolidationStateView(state)
	view.Brief = true
	result.Consolidation = view
	result.Warnings = append(result.Warnings, consolidationWarnings(state)...)
}

// dueConsolidationWarning returns the due-consolidation warning, or "".
func dueConsolidationWarning(root string) string {
	inputs, err := consolidate.LoadInputs(root)
	if err != nil {
		return ""
	}
	return dueWarning(inputs.State())
}

// coherenceRequest asks for the semantic check every pending change needs,
// so the request reaches the agent even when the user only asks to consolidate.
const coherenceRequest = "compare each pending change with the statements linked to it in the report's comparisons"

func dueWarning(state consolidate.State) string {
	if state.Tracking != consolidate.TrackingActive || state.Threshold != consolidate.ThresholdDue {
		return ""
	}
	return fmt.Sprintf("consolidation due: %d contract changes since the last consolidation (%s) — run walden consolidate and %s", len(state.Pending), pendingList(state), coherenceRequest)
}

// consolidationWarnings lists the warnings every consolidation-aware command
// shares: suggested or due consolidation, a stale view and an unknown state.
func consolidationWarnings(state consolidate.State) []string {
	warnings := []string{}
	switch {
	case state.Tracking == consolidate.TrackingUnknown:
		return append(warnings, fmt.Sprintf("consolidation state unknown: %s — %s", state.Problem, state.Remedy))
	case state.Threshold == consolidate.ThresholdDue:
		warnings = append(warnings, dueWarning(state))
	case state.Threshold == consolidate.ThresholdSuggested:
		warnings = append(warnings, fmt.Sprintf("consolidation suggested: %d contract changes since the last consolidation (%s) — run walden consolidate and %s", len(state.Pending), pendingList(state), coherenceRequest))
	}
	if state.View.State == consolidate.ViewStale {
		warnings = append(warnings, fmt.Sprintf("the current-contract view %s is stale: it changed after approval — review it, then walden consolidate open", consolidate.ViewPath))
	}
	return warnings
}

func consolidationSummary(state consolidate.State) string {
	switch state.Tracking {
	case consolidate.TrackingNotStarted:
		return fmt.Sprintf("consolidation tracking has not started: %d approved feature(s) are not consolidated", state.Unconsolidated)
	case consolidate.TrackingUnknown:
		return "consolidation state unknown: " + state.Problem
	}
	summary := fmt.Sprintf("%d contract change(s) pending since the last consolidation; %d feature(s) in the backlog", len(state.Pending), len(state.Backlog))
	switch state.Threshold {
	case consolidate.ThresholdDue:
		summary += "; consolidation due"
	case consolidate.ThresholdSuggested:
		summary += "; consolidation suggested"
	}
	return summary
}

func consolidationNextAction(state consolidate.State) string {
	switch {
	case state.Tracking == consolidate.TrackingNotStarted:
		return "walden consolidate start"
	case state.Tracking == consolidate.TrackingUnknown:
		return state.Remedy
	case state.View.State == consolidate.ViewInReview:
		return "present " + consolidate.ViewPath + " to the user; after explicit approval run walden consolidate approve"
	case state.View.State == consolidate.ViewStale:
		return "review " + consolidate.ViewPath + ", then walden consolidate open"
	case len(state.Pending) > 0:
		return coherenceRequest + ", record the outcome per pending feature under " + consolidate.CoherenceHeading + " in " + consolidate.ViewPath +
			" citing the linked statements you compared as `feature#ID`, review the rest of the scope, propose fixes through normal reviews, update the view's feature sections, then walden consolidate open"
	}
	return ""
}
