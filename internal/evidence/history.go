package evidence

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
)

// ResolvePlans adds only requested, locally available historical witnesses.
// Current bodies are already supplied by FeatureInputs. Results and gaps are
// cached in this invocation's inputs; no history index or ledger is written.
// A supplied revision pins release diagnostics to its captured commit.
func ResolvePlans(ctx context.Context, runner shell.Runner, root string, feature spec.Feature, ledger Document, current *ChainFingerprints, revision string) {
	if current.Plans == nil {
		current.Plans = map[string]PlanWitness{}
	}
	if current.PlanGaps == nil {
		current.PlanGaps = map[string]string{}
	}
	needed := map[string]bool{}
	for _, record := range ledger.Tasks {
		if record.TaskFingerprintScheme != "" {
			continue
		}
		fingerprint := record.TasksFingerprint
		if _, found := current.Plans[fingerprint]; found {
			continue
		}
		if _, found := current.PlanGaps[fingerprint]; found {
			continue
		}
		if !spec.ValidFingerprint(fingerprint) {
			current.PlanGaps[fingerprint] = "historical full-plan fingerprint is missing or malformed"
			continue
		}
		needed[fingerprint] = true
	}
	if len(needed) == 0 {
		return
	}
	gap := func(reason string) {
		for fingerprint := range needed {
			current.PlanGaps[fingerprint] = reason
		}
	}
	path, err := filepath.Rel(root, feature.Tasks.Path)
	if err != nil || !filepath.IsLocal(path) {
		gap("historical task path is not local to this repository")
		return
	}
	path = filepath.ToSlash(path)
	if revision == "" {
		revision, err = Commit(ctx, runner, root, "HEAD")
		if err != nil {
			gap("historical plan unavailable: " + err.Error())
			return
		}
	}
	if !objectID(revision) {
		gap("historical lookup requires a captured commit object id")
		return
	}
	log, err := Git(ctx, runner, root, "log", "--format=%H", "--full-history", revision, "--", path)
	if err != nil || log.ExitCode != 0 {
		gap(fmt.Sprintf("historical plan lookup unavailable: %v; %s", err, strings.TrimSpace(log.Stderr)))
		return
	}
	seen := map[string]bool{}
	var readProblem string
	for _, commit := range strings.Fields(log.Stdout) {
		if ctx.Err() != nil {
			gap("historical lookup canceled: " + ctx.Err().Error())
			return
		}
		if !objectID(commit) {
			readProblem = "invalid historical commit id"
			continue
		}
		listing, err := Git(ctx, runner, root, "ls-tree", "-z", commit, "--", path)
		if err != nil || listing.ExitCode != 0 {
			readProblem = "historical tree unreadable"
			continue
		}
		entries, err := ParseTree(listing.Stdout)
		if err != nil {
			readProblem = err.Error()
			continue
		}
		for _, entry := range entries {
			if entry.Path != path || entry.Kind != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
				readProblem = "historical plan is not a regular blob"
				continue
			}
			if seen[entry.OID] {
				continue
			}
			seen[entry.OID] = true
			blob, err := Git(ctx, runner, root, "cat-file", "blob", entry.OID)
			if err != nil || blob.ExitCode != 0 {
				readProblem = "historical blob unreadable"
				continue
			}
			body, err := spec.HistoricalBody([]byte(blob.Stdout))
			if err != nil {
				readProblem = "historical frontmatter cannot be parsed"
				continue
			}
			fingerprint := spec.Fingerprint(path, body)
			if !needed[fingerprint] {
				continue
			}
			delete(needed, fingerprint)
			tree, err := spec.ParseTaskTree(spec.Document{Exists: true, Path: path, Body: body})
			if err != nil {
				current.PlanGaps[fingerprint] = "fingerprint-bound historical proof cannot be parsed: " + err.Error()
			} else {
				witness := PlanWitness{Fingerprint: fingerprint, Path: path, Commit: commit, Source: "git-plan", Tasks: map[string]*spec.Task{}}
				for _, task := range tree.LeafTasks() {
					witness.Tasks[task.ID] = task
				}
				current.Plans[fingerprint] = witness
			}
			if len(needed) == 0 {
				return
			}
		}
	}
	reason := "fingerprint-bound plan not found in available local history; shallow or uncommitted history cannot supply a witness"
	if readProblem != "" {
		reason += "; " + readProblem
	}
	gap(reason)
}
