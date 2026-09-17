package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/validation"
	"github.com/andrearaponi/walden/internal/workflow"
)

// gitRunner executes the gate's read-only git plumbing. It is a seam so
// tests control worktree and identity outcomes.
var gitRunner shell.Runner = shell.NewExecRunner()

// decisionMarker is the skill protocol's open-checkpoint marker. The gate
// treats it as a deterministic substring in approved documents — a textual
// lint, never an interpretation of the protocol itself.
const decisionMarker = "[decision:"

// CriterionResult is one certification criterion's outcome for a feature.
type CriterionResult struct {
	Name     string
	Passed   bool
	Blockers []string
}

// FeatureCertification is one feature's full certification record.
type FeatureCertification struct {
	Feature  string
	Criteria []CriterionResult
	Pending  []string
	Evidence []evidence.TaskEvidence
}

// Options are the certification policy knobs. The zero value is the default
// policy: plans-complete required, uncommitted .walden/ tolerated.
type Options struct {
	// Strict requires committed .walden/ state: a final certification must
	// be reproducible from the commit it judges.
	Strict bool
	// AllowPending waives pending leaf tasks for this verdict. The surface
	// guarantees a non-empty WaiverReason accompanies it; the report carries
	// both — the verdict is the waiver's durable record.
	AllowPending bool
	WaiverReason string
}

// ReleaseReport is the aggregate verdict: releasable iff zero blockers.
type ReleaseReport struct {
	Scope            evidence.Scope
	Features         []FeatureCertification
	WorktreeBlockers []string
	WaldenDirty      []string
	GitSkipped       bool
	Strict           bool
	AllowPending     bool
	WaiverReason     string
	// CertifiedCommit is the HEAD revision the certification ran against —
	// the commit an auditor checks out. Empty when git is unusable (already
	// a repository-level blocker) or HEAD is unborn.
	CertifiedCommit       string
	CommittedInputBinding string
	Inputs                []InputBinding
}

// BlockerCount sums every blocker across features and the worktree.
func (r ReleaseReport) BlockerCount() int {
	count := len(r.WorktreeBlockers)
	for _, feature := range r.Features {
		for _, criterion := range feature.Criteria {
			count += len(criterion.Blockers)
		}
	}
	return count
}

// Releasable reports the verdict.
func (r ReleaseReport) Releasable() bool { return r.BlockerCount() == 0 }

// Completion classes: the verdict's answer to "was anything still planned
// when this certification ran, and was it waived?".
const (
	CompletionComplete    = "complete"
	CompletionWithPending = "with-pending"
	CompletionWithWaivers = "with-waivers"
)

// Completion derives the completion class from the certified features'
// pending lists and the active waiver — a reading of report state, never
// stored input, so it can never disagree with the data it summarizes.
func (r ReleaseReport) Completion() string {
	pending := false
	for _, feature := range r.Features {
		if len(feature.Pending) > 0 {
			pending = true
			break
		}
	}
	switch {
	case !pending:
		return CompletionComplete
	case r.AllowPending:
		return CompletionWithWaivers
	default:
		return CompletionWithPending
	}
}

// WaivedTasks derives the feature-qualified identifiers this verdict waived
// — derived like Completion, never stored, empty without an active waiver.
func (r ReleaseReport) WaivedTasks() []string {
	if !r.AllowPending {
		return nil
	}
	waived := []string{}
	for _, feature := range r.Features {
		for _, taskID := range feature.Pending {
			waived = append(waived, feature.Feature+": "+taskID)
		}
	}
	return waived
}

// ReleaseCheck certifies one feature (or, with an empty name, every feature
// under .walden/specs/ in sorted order) against the release criteria. It
// judges existing truths — chain freshness, validation, decision markers,
// evidence — and executes no proofs and persists nothing: verify produces
// evidence, release check judges it.
func ReleaseCheck(ctx context.Context, root, featureName string, opts Options) (ReleaseReport, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return ReleaseReport{}, fmt.Errorf("resolve repository root: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return ReleaseReport{}, err
	}
	features, err := releaseTargets(root, featureName)
	if err != nil {
		return ReleaseReport{}, err
	}
	commit, commitErr := evidence.Commit(ctx, gitRunner, root, "HEAD")
	report := ReleaseReport{Scope: evidence.NewScope(featureName != "", features...), Strict: opts.Strict, AllowPending: opts.AllowPending, WaiverReason: opts.WaiverReason,
		CertifiedCommit: commit, CommittedInputBinding: "not-requested"}
	var manifest evidence.Manifest
	var identityOK bool
	if commit != "" {
		manifest, identityOK = evidence.CaptureManifestAtCommit(ctx, gitRunner, root, commit)
	} else {
		manifest, identityOK = evidence.CaptureManifest(ctx, gitRunner, root)
	}
	identity := ""
	if identityOK {
		identity = manifest.Digest()
	}
	var inputs []inputSnapshot
	for _, name := range features {
		snapshot := captureFeature(root, name, opts.Strict)
		inputs = append(inputs, snapshot.Inputs...)
		report.Features = append(report.Features, certifyFeature(ctx, root, snapshot, opts, identity, identityOK, commit))
	}
	report.WorktreeBlockers, report.WaldenDirty, report.GitSkipped = worktreeCriterion(ctx, root)
	if opts.Strict {
		report.CommittedInputBinding = "matched"
		if commitErr != nil {
			report.WorktreeBlockers = append(report.WorktreeBlockers, fmt.Sprintf("HEAD commit unavailable: %v — create a commit before certifying (--strict)", commitErr))
			report.CommittedInputBinding = "blocked"
		}
		for _, input := range inputs {
			report.Inputs = append(report.Inputs, bindInput(ctx, root, commit, input))
		}
		if featureName == "" {
			report.Inputs = append(report.Inputs, bindPortfolio(ctx, root, commit, features))
		}
		for _, binding := range report.Inputs {
			if binding.State != "matched" && binding.State != "matched-absent" {
				report.CommittedInputBinding = "blocked"
				report.WorktreeBlockers = append(report.WorktreeBlockers, inputBlocker(binding))
			}
		}
		for _, path := range report.WaldenDirty {
			report.WorktreeBlockers = append(report.WorktreeBlockers, fmt.Sprintf("uncommitted under .walden/: %s — commit it before certifying (--strict)", path))
		}
	}
	if commit != "" {
		if final, err := evidence.Commit(ctx, gitRunner, root, "HEAD"); err != nil || final != commit {
			report.WorktreeBlockers = append(report.WorktreeBlockers, "HEAD changed or became unreadable during judgment — retry against a stable commit")
		}
	}
	if report.GitSkipped || !identityOK {
		report.WorktreeBlockers = append(report.WorktreeBlockers, "no usable git repository — certification requires a git-backed code identity; initialize git, commit, and rerun")
	}
	sort.Strings(report.WorktreeBlockers)
	return report, nil
}

// releaseTargets resolves the feature list for the run.
func releaseTargets(root, featureName string) ([]string, error) {
	if featureName != "" {
		normalized, err := spec.NormalizeFeatureName(featureName)
		if err != nil {
			return nil, err
		}
		return []string{normalized}, nil
	}

	entries, err := os.ReadDir(filepath.Join(root, ".walden", "specs"))
	if err != nil {
		return nil, fmt.Errorf("read feature specs: %w", err)
	}
	features := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			features = append(features, entry.Name())
		}
	}
	sort.Strings(features)
	if len(features) == 0 {
		return nil, fmt.Errorf("no features under .walden/specs to certify")
	}
	return features, nil
}

// certifyFeature evaluates every criterion; nothing short-circuits — a
// certification is a complete work list, not a first failure.
func certifyFeature(ctx context.Context, root string, snapshot featureSnapshot, opts Options, identity string, identityOK bool, commit string) FeatureCertification {
	feature, loadErr := snapshot.Feature, snapshot.LoadErr
	name := feature.Name
	certification := FeatureCertification{Feature: name}

	// Criterion 1: the approval chain, approved and fresh.
	chain := CriterionResult{Name: "chain"}
	if loadErr != nil {
		chain.Blockers = append(chain.Blockers, loadErr.Error())
	} else if readiness, err := workflow.ResolveExecutionReadiness(feature); err != nil {
		chain.Blockers = append(chain.Blockers, err.Error())
	} else if readiness.GateBlocked {
		chain.Blockers = append(chain.Blockers, readiness.Blockers...)
	}
	chain.Passed = len(chain.Blockers) == 0
	certification.Criteria = append(certification.Criteria, chain)

	// Criterion 2: full-spec validation.
	valid := CriterionResult{Name: "validation"}
	if loadErr != nil {
		valid.Blockers = append(valid.Blockers, loadErr.Error())
	} else if result, err := validation.ValidateLoadedFeature(feature, validation.ScopeFullSpec); err != nil {
		valid.Blockers = append(valid.Blockers, fmt.Sprintf("%v — fix and rerun walden validate %s --all", err, name))
	} else if !result.Valid {
		valid.Blockers = append(valid.Blockers, fmt.Sprintf("%s — fix and rerun walden validate %s --all", result.Message, name))
	}
	valid.Passed = len(valid.Blockers) == 0
	certification.Criteria = append(certification.Criteria, valid)

	// Criterion 3: no unresolved decision markers in approved documents.
	// Drafts of in-flight features may legitimately carry open checkpoints.
	decisions := CriterionResult{Name: "decisions"}
	if loadErr == nil {
		for _, doc := range []struct {
			name     string
			document spec.Document
		}{
			{"requirements.md", feature.Requirements},
			{"design.md", feature.Design},
			{"tasks.md", feature.Tasks},
		} {
			if !doc.document.Exists || doc.document.Status != "approved" {
				continue
			}
			stripped, terminated := stripHTMLComments(doc.document.Body)
			if !terminated {
				// The dangling opener would swallow the rest of the document
				// from the scan — and any marker hidden inside it.
				decisions.Blockers = append(decisions.Blockers, fmt.Sprintf("unterminated HTML comment in %s — close it with --> and re-approve", doc.name))
			}
			if strings.Contains(stripped, decisionMarker) {
				decisions.Blockers = append(decisions.Blockers, fmt.Sprintf("unresolved %s] marker in %s — resolve it and re-approve", decisionMarker, doc.name))
			}
		}
	}
	decisions.Passed = len(decisions.Blockers) == 0
	certification.Criteria = append(certification.Criteria, decisions)

	// Criterion 4: executed work must be verified; planned work informs.
	evidenceCriterion := CriterionResult{Name: "evidence"}
	if loadErr == nil {
		if tree, err := spec.ParseTaskTree(feature.Tasks); err == nil {
			ledger := snapshot.Ledger
			if snapshot.LedgerErr != nil {
				evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("%v — retain the ledger and inspect it with a compatible reader", snapshot.LedgerErr))
			} else {
				current, leafs := evidence.FeatureInputs(feature, tree)
				evidence.ResolvePlans(ctx, gitRunner, root, feature, ledger, &current, commit)
				for _, derived := range evidence.Derive(ledger, current, identity, identityOK, leafs) {
					certification.Evidence = append(certification.Evidence, derived)
					switch derived.State {
					case evidence.StateVerified:
					case evidence.StatePending:
						certification.Pending = append(certification.Pending, derived.TaskID)
						// The flip: an unexecuted plan blocks certification
						// unless explicitly waived — the plan is a promise
						// the release must keep or visibly defer.
						if !opts.AllowPending {
							evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("task %s is pending — execute it, or waive with --allow-pending --reason", derived.TaskID))
						}
					default:
						evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("task %s is %s: %s — inspect walden adopt %s; request applicable proofs with walden verify %s", derived.TaskID, derived.State, derived.GapSummary(), name, name))
					}
				}
			}
		} else {
			evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, err.Error())
		}
	}
	evidenceCriterion.Passed = len(evidenceCriterion.Blockers) == 0
	certification.Criteria = append(certification.Criteria, evidenceCriterion)

	return certification
}

// stripHTMLComments removes well-formed `<!-- ... -->` spans before the
// decision scan: assumed markers live in comments by skill convention and
// legitimately mention the decision marker in prose. The boolean is false
// when an opener never terminates — the caller must fail closed rather than
// trust a scan that cannot see the swallowed remainder.
func stripHTMLComments(body string) (string, bool) {
	var kept strings.Builder
	for {
		start := strings.Index(body, "<!--")
		if start < 0 {
			kept.WriteString(body)
			return kept.String(), true
		}
		kept.WriteString(body[:start])
		end := strings.Index(body[start:], "-->")
		if end < 0 {
			return kept.String(), false
		}
		body = body[start+end+len("-->"):]
	}
}

// worktreeCriterion partitions uncommitted paths once per run: code outside
// .walden/ blocks certification (what you certify must be what you tag),
// .walden/ itself warns by default — a freshly refreshed ledger legitimately
// precedes its own commit — and is promoted to blockers by the caller under
// strict. Unusable git is reported; the caller fails the run closed.
func worktreeCriterion(ctx context.Context, root string) (blockers, waldenDirty []string, skipped bool) {
	status, err := evidence.Git(ctx, gitRunner, root, "status", "--porcelain", "-z")
	if err != nil || status.ExitCode != 0 {
		return nil, nil, true
	}

	for _, path := range porcelainPathList(status.Stdout) {
		if path == ".walden" || strings.HasPrefix(path, ".walden/") {
			waldenDirty = append(waldenDirty, path)
			continue
		}
		blockers = append(blockers, fmt.Sprintf("uncommitted: %s — commit it before certifying", path))
	}
	sort.Strings(blockers)
	sort.Strings(waldenDirty)
	return blockers, waldenDirty, false
}

// porcelainPathList extracts worktree paths from NUL-separated porcelain
// output; rename and copy entries carry the source as a separate field,
// consumed without being reported.
func porcelainPathList(stdout string) []string {
	fields := strings.Split(stdout, "\x00")
	paths := []string{}
	for index := 0; index < len(fields); index++ {
		entry := fields[index]
		if len(entry) < 4 {
			continue
		}
		code, path := entry[:2], entry[3:]
		if strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C") {
			index++
		}
		paths = append(paths, path)
	}
	return paths
}
