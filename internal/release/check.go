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
}

// ReleaseReport is the aggregate verdict: releasable iff zero blockers.
type ReleaseReport struct {
	Features         []FeatureCertification
	WorktreeBlockers []string
	WaldenDirty      []string
	GitSkipped       bool
	Strict           bool
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

// ReleaseCheck certifies one feature (or, with an empty name, every feature
// under .walden/specs/ in sorted order) against the release criteria. It
// judges existing truths — chain freshness, validation, decision markers,
// evidence — and executes no proofs and persists nothing: verify produces
// evidence, release check judges it.
func ReleaseCheck(ctx context.Context, root, featureName string, strict bool) (ReleaseReport, error) {
	features, err := releaseTargets(root, featureName)
	if err != nil {
		return ReleaseReport{}, err
	}

	// One identity for the whole run: every feature's evidence is judged
	// against the same tree.
	identity, identityOK := evidence.Identity(ctx, gitRunner, root)

	report := ReleaseReport{Strict: strict}
	for _, name := range features {
		report.Features = append(report.Features, certifyFeature(root, name, strict, identity, identityOK))
	}

	report.WorktreeBlockers, report.WaldenDirty, report.GitSkipped = worktreeCriterion(ctx, root)
	if strict {
		// Strict certification is plans-complete and final: the .walden/
		// state the verdict judged must exist in the commit being certified.
		for _, path := range report.WaldenDirty {
			report.WorktreeBlockers = append(report.WorktreeBlockers, fmt.Sprintf("uncommitted under .walden/: %s — commit it before certifying (--strict)", path))
		}
	}
	if report.GitSkipped || !identityOK {
		// A verdict without a code identity cannot name the tree it
		// certified: fail closed instead of judging blind.
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
func certifyFeature(root, name string, strict bool, identity string, identityOK bool) FeatureCertification {
	certification := FeatureCertification{Feature: name}

	feature, loadErr := spec.LoadFeature(root, name)

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
	if result, err := validation.ValidateFeatureWithScope(root, name, validation.ScopeFullSpec); err != nil {
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
			ledger, err := evidence.Load(root, name)
			if err != nil {
				evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("%v — remove the file and run walden verify %s --all", err, name))
			} else {
				leafs := []evidence.LeafTask{}
				for _, task := range tree.LeafTasks() {
					leafs = append(leafs, evidence.LeafTask{
						ID:          task.ID,
						Completed:   task.Completed,
						Fingerprint: spec.TaskDefinitionFingerprint(task),
					})
				}
				current := evidence.ChainFingerprints{
					Requirements: feature.Requirements.Fields["approved_fingerprint"],
					Design:       feature.Design.Fields["approved_fingerprint"],
				}
				for _, derived := range evidence.Derive(ledger, current, identity, identityOK, leafs) {
					switch derived.State {
					case evidence.StateVerified:
					case evidence.StatePending:
						certification.Pending = append(certification.Pending, derived.TaskID)
						if strict {
							evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("task %s not executed (--strict) — complete it or drop --strict", derived.TaskID))
						}
					default:
						evidenceCriterion.Blockers = append(evidenceCriterion.Blockers, fmt.Sprintf("task %s is %s — run walden verify %s", derived.TaskID, derived.State, name))
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
	status, err := gitRunner.Run(ctx, "git", "-C", root, "status", "--porcelain", "-z")
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
