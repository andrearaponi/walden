package app

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/testutil"
)

// setupPendingProof bootstraps a real repository in a temp cwd, approves a
// one-task feature and leaves its task to complete.
func setupPendingProof(t *testing.T) {
	t.Helper()
	root := chdirContract(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"repo", "init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("repo init failed: %s", stderr.String())
	}
	if code := Run([]string{"feature", "init", "Todo App Demo"}, &stdout, &stderr); code != 0 {
		t.Fatalf("feature init failed: %s", stderr.String())
	}
	writeStatusFeatureFile(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T07:00:00Z
last_modified: 2026-03-22T07:00:00Z
---

# Requirements Document
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-22T07:10:00Z
last_modified: 2026-03-22T07:10:00Z
source_requirements_approved_at: 2026-03-22T07:00:00Z
---

# Feature Design
`)
	writeStatusFeatureFile(t, root, "todo-app-demo", "tasks.md", `---
status: approved
approved_at: 2026-03-22T07:20:00Z
last_modified: 2026-03-22T07:20:00Z
source_design_approved_at: 2026-03-22T07:10:00Z
---

# Implementation Plan

- [ ] 1. Build parser
  - Requirements: `+"`R1`"+`
  - Design: Task Store
  - Verification:
    - command: ["go", "test", "./internal/spec"]
`)
}

// proofFailureResult runs a command expected to fail under one scripted proof
// response and decodes its JSON result.
func proofFailureResult(t *testing.T, response testutil.Response, args ...string) map[string]json.RawMessage {
	t.Helper()
	overrideCommandRunner(t, testutil.NewFakeRunner(response))

	var stdout, stderr bytes.Buffer
	if code := Run(append(args, "--json"), &stdout, &stderr); code == 0 {
		t.Fatalf("%v unexpectedly succeeded: %s", args, stdout.String())
	}
	var envelope struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %v: %v\n%s", args, err, stdout.String())
	}
	return envelope.Result
}

func sortedKeys[V any](fields map[string]V) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestProofFailureExcerptAddsNoJSONField(t *testing.T) {
	const sentinel = "PROOF-OUTPUT-SENTINEL"
	silent := testutil.Response{ExitCode: 1}
	noisy := testutil.Response{Stdout: sentinel + "\n", Stderr: "noise\n", ExitCode: 1}

	assertSameKeys := func(t *testing.T, quiet, loud []string) {
		t.Helper()
		if !slices.Equal(quiet, loud) {
			t.Fatalf("the excerpt changed the JSON fields:\nsilent: %v\n noisy: %v", quiet, loud)
		}
	}

	t.Run("task complete", func(t *testing.T) {
		setupPendingProof(t)
		args := []string{"task", "complete", "todo-app-demo", "1"}
		quiet := proofFailureResult(t, silent, args...)
		loud := proofFailureResult(t, noisy, args...)

		assertSameKeys(t, sortedKeys(quiet), sortedKeys(loud))
		if !strings.Contains(string(loud["summary"]), sentinel) {
			t.Fatalf("summary does not report the output: %s", loud["summary"])
		}
	})

	t.Run("verify", func(t *testing.T) {
		setupEvidenceFeature(t)
		args := []string{"verify", "todo-app-demo", "--all"}
		entry := func(result map[string]json.RawMessage) map[string]json.RawMessage {
			t.Helper()
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(result["evidence"], &entries); err != nil || len(entries) != 1 {
				t.Fatalf("decode evidence: %v\n%s", err, result["evidence"])
			}
			return entries[0]
		}
		quiet := proofFailureResult(t, silent, args...)
		loud := proofFailureResult(t, noisy, args...)

		assertSameKeys(t, sortedKeys(quiet), sortedKeys(loud))
		assertSameKeys(t, sortedKeys(entry(quiet)), sortedKeys(entry(loud)))
		if !strings.Contains(string(entry(loud)["failure"]), sentinel) {
			t.Fatalf("failure does not report the output: %s", entry(loud)["failure"])
		}
	})
}

func TestProofFailureProposesNoFeatureInit(t *testing.T) {
	// Both words the hint used to look for, printed by a proof of a feature
	// that exists.
	misleading := testutil.Response{Stderr: "config file does not exist\nfeature flags unavailable\n", ExitCode: 1}

	for _, command := range [][]string{
		{"task", "complete", "todo-app-demo", "1"},
		{"task", "complete-all", "todo-app-demo"},
	} {
		t.Run(strings.Join(command[:2], " "), func(t *testing.T) {
			setupPendingProof(t)
			result := proofFailureResult(t, misleading, command...)

			if !strings.Contains(string(result["summary"]), "does not exist") {
				t.Fatalf("summary lost the proof output: %s", result["summary"])
			}
			if strings.Contains(string(result["summary"]), "walden feature init") {
				t.Fatalf("summary proposes initializing an existing feature: %s", result["summary"])
			}
			if strings.Contains(string(result["next_action"]), "walden feature init") {
				t.Fatalf("next action proposes initializing an existing feature: %s", result["next_action"])
			}
		})
	}
}

func TestMissingFeatureKeepsInitHint(t *testing.T) {
	for _, command := range [][]string{
		{"task", "complete", "ghost-feature", "1"},
		{"task", "complete-all", "ghost-feature"},
	} {
		t.Run(strings.Join(command[:2], " "), func(t *testing.T) {
			setupPendingProof(t)
			result := proofFailureResult(t, testutil.Response{}, command...)

			wantSummary := `feature \"ghost-feature\" does not exist; run ` + "`walden feature init ghost-feature`" + ` to initialize it`
			if !strings.Contains(string(result["summary"]), wantSummary) {
				t.Fatalf("summary lost the hint: %s", result["summary"])
			}
			if !strings.Contains(string(result["next_action"]), "Run walden feature init ghost-feature") {
				t.Fatalf("next action lost the hint: %s", result["next_action"])
			}
		})
	}
}
