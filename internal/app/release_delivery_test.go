package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

const deliveryReleaseFeature = "delivery-release"

// deliveryReleaseTasks holds two delivery leaves and a regression leaf. Only
// a delivery proof covers R1.AC1; a delivery proof and the regression proof
// both cover R1.AC2.
const deliveryReleaseTasks = `---
status: draft
approved_at:
last_modified: 2026-07-11T13:00:00Z
approved_fingerprint:
source_design_approved_at:
source_design_fingerprint:
---

# Implementation Plan

- [ ] 1. Markers
  - [ ] 1.1 Publish the notes
    - Requirements: ` + "`R1.AC1`" + `
    - Design: Overview
    - Proof: delivery
    - Verification:
      - command: ["grep", "-q", "released", "notes.txt"]
        covers: ["R1.AC1"]
  - [ ] 1.2 Announce the notes
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Proof: delivery
    - Verification:
      - command: ["grep", "-q", "announced", "notes.txt"]
        covers: ["R1.AC2"]
  - [ ] 1.3 Keep the marker
    - Requirements: ` + "`R1.AC2`" + `
    - Design: Overview
    - Verification:
      - command: ["sh", "-c", "true"]
        covers: ["R1.AC2"]
`

// deliveredReleaseRepo is an isolated, committed repository whose feature is
// certifiable: its two delivery tasks were completed on the committed tree,
// so both are attested at the first commit, its regression task is verified,
// and the evidence is committed.
func deliveredReleaseRepo(t *testing.T) (root, attested string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root = chdirContract(t)
	diagnosticsFile(t, root, "main.go", "package main\n")
	diagnosticsFile(t, root, "notes.txt", "released and announced\n")
	addDeliveryFeature(t, root, deliveryReleaseFeature, deliveryReleaseTasks)
	gitify(t, root)
	attested = integrityGit(t, root, "rev-parse", "HEAD")
	for _, id := range []string{"1.1", "1.2", "1.3"} {
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		integrityMustCLI(t, "task", "complete", deliveryReleaseFeature, id)
	}
	integrityGit(t, root, "add", "-A")
	integrityGit(t, root, "commit", "-qm", "evidence")
	return root, attested
}

// releaseBoth runs `walden release check` on the delivery feature in JSON and
// in text mode. The verdict may be blocked: both outputs are returned as
// they are.
func releaseBoth(t *testing.T, feature string) (result map[string]any, text string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	Run([]string{"release", "check", feature, "--json"}, &stdout, &stderr)
	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("release check --json printed no envelope: %v (%s%s)", err, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	Run([]string{"release", "check", feature}, &stdout, &stderr)
	return envelope.Result, stdout.String() + stderr.String()
}

func releaseFeature(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	release, _ := result["release"].(map[string]any)
	features, _ := release["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("the verdict holds %d feature(s), want one: %v", len(features), result)
	}
	return features[0].(map[string]any)
}

func releaseEntries(t *testing.T, result map[string]any) map[string]map[string]any {
	t.Helper()
	entries := map[string]map[string]any{}
	list, _ := releaseFeature(t, result)["evidence"].([]any)
	for _, item := range list {
		entry := item.(map[string]any)
		entries[entry["task_id"].(string)] = entry
	}
	return entries
}

// sealAttestation marks the attestation of a task as sealed, as adoption
// would have written it.
func sealAttestation(t *testing.T, root, feature, id string) {
	t.Helper()
	path := evidence.DocumentPath(root, feature)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	document["tasks"].(map[string]any)[id].(map[string]any)["attestation"].(map[string]any)["sealed"] = true
	data, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write the ledger: %v", err)
	}
}

func TestReleaseCheckShowsDelivered(t *testing.T) {
	t.Run("entries", func(t *testing.T) {
		_, attested := deliveredReleaseRepo(t)
		result, text := releaseBoth(t, deliveryReleaseFeature)
		if releasable := result["release"].(map[string]any)["releasable"]; releasable != true {
			t.Fatalf("the verdict is not releasable: %v", result["blockers"])
		}
		entries := releaseEntries(t, result)
		for _, id := range []string{"1.1", "1.2"} {
			entry := entries[id]
			attestation, _ := entry["attestation"].(map[string]any)
			if entry["state"] != "delivered" || entry["delivery"] != true || attestation["commit"] != attested || attestation["sealed"] != false {
				t.Errorf("task %s: %v, want delivered at %s", id, entry, attested)
			}
			// The text names it with its commit and never as verified.
			if want := "  - " + id + ": delivered at " + attested + " ["; !strings.Contains(text, want) {
				t.Errorf("the text does not hold %q:\n%s", want, text)
			}
			if strings.Contains(text, "  - "+id+": verified") {
				t.Errorf("the text shows the delivered task %s as verified:\n%s", id, text)
			}
		}
		if entry := entries["1.3"]; entry["state"] != "verified" || entry["delivery"] != nil || entry["attestation"] != nil {
			t.Errorf("the regression task: %v", entry)
		}
		if !strings.Contains(text, "  - 1.3: verified [") {
			t.Errorf("the text does not show the regression task as verified:\n%s", text)
		}
	})

	t.Run("sealed", func(t *testing.T) {
		root, attested := deliveredReleaseRepo(t)
		sealAttestation(t, root, deliveryReleaseFeature, "1.1")
		result, text := releaseBoth(t, deliveryReleaseFeature)
		entries := releaseEntries(t, result)
		if sealed := entries["1.1"]["attestation"].(map[string]any)["sealed"]; sealed != true {
			t.Errorf("task 1.1: sealed = %v, want true", sealed)
		}
		if sealed := entries["1.2"]["attestation"].(map[string]any)["sealed"]; sealed != false {
			t.Errorf("task 1.2: sealed = %v, want false", sealed)
		}
		if want := "  - 1.1: delivered at " + attested + " (sealed by adoption) ["; !strings.Contains(text, want) {
			t.Errorf("the text does not hold %q:\n%s", want, text)
		}
		if strings.Count(text, "(sealed by adoption)") != 1 {
			t.Errorf("the sealing mark is on the wrong entries:\n%s", text)
		}
	})

	t.Run("summary", func(t *testing.T) {
		root, _ := deliveredReleaseRepo(t)
		result, text := releaseBoth(t, deliveryReleaseFeature)
		summary, _ := result["summary"].(string)
		want := regexp.MustCompile(`^RELEASABLE — 1 feature\(s\) certified \[feature: ` + deliveryReleaseFeature + `\], 2 delivered task\(s\) not proven again, commit [0-9a-f]{12}$`)
		if !want.MatchString(summary) || !strings.Contains(text, "Summary: "+summary+"\n") {
			t.Errorf("summary %q does not state the two delivered tasks", summary)
		}

		// A blocked verdict is worded as before: it states no count.
		diagnosticsFile(t, root, "notes.txt", "superseded\n")
		integrityGit(t, root, "commit", "-qam", "the notes move on")
		result, _ = releaseBoth(t, deliveryReleaseFeature)
		if summary, _ := result["summary"].(string); summary != "NOT RELEASABLE — 1 blocker(s) [feature: "+deliveryReleaseFeature+"]" {
			t.Errorf("blocked summary %q", summary)
		}
		if blockers, _ := result["blockers"].([]any); len(blockers) != 1 || !strings.Contains(blockers[0].(string), "task 1.3 is stale-code") {
			t.Errorf("blockers = %v, want the regression task alone", result["blockers"])
		}

		// A releasable verdict without delivered tasks is worded as before.
		setupReleasableFeature(t, verifyTaskFeature)
		overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"}))
		integrityMustCLI(t, "task", "complete-all", verifyTaskFeature)
		integrityGit(t, ".", "add", "-A")
		integrityGit(t, ".", "commit", "-qm", "evidence")
		result, text = releaseBoth(t, verifyTaskFeature)
		plain := regexp.MustCompile(`^RELEASABLE — 1 feature\(s\) certified \[feature: ` + verifyTaskFeature + `\], commit [0-9a-f]{12}$`)
		if summary, _ := result["summary"].(string); !plain.MatchString(summary) {
			t.Errorf("summary without delivered tasks %q", summary)
		}
		if strings.Contains(text, "delivered") || strings.Contains(text, "only delivery proofs cover") {
			t.Errorf("a feature without delivery proofs mentions them:\n%s", text)
		}
		if _, present := releaseFeature(t, result)["delivery_only"]; present {
			t.Errorf("a feature without delivery proofs carries delivery_only")
		}
	})

	t.Run("delivery-only", func(t *testing.T) {
		deliveredReleaseRepo(t)
		result, text := releaseBoth(t, deliveryReleaseFeature)
		only, _ := releaseFeature(t, result)["delivery_only"].([]any)
		if len(only) != 1 || only[0] != "R1.AC1" {
			t.Errorf("delivery_only = %v, want R1.AC1 alone", releaseFeature(t, result)["delivery_only"])
		}
		if want := "  only delivery proofs cover: R1.AC1\n"; !strings.Contains(text, want) {
			t.Errorf("the text does not hold %q:\n%s", want, text)
		}
	})
}
