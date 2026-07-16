package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestReleaseCheckWaiverFlags(t *testing.T) {
	setupReleasableFeature(t, "gate-demo")

	t.Run("allow-pending requires a reason", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"release", "check", "--allow-pending", "--json"}, &stdout, &stderr); code == 0 {
			t.Fatal("--allow-pending accepted without --reason")
		}
		envelope := decodeReleaseEnvelope(t, &stdout)
		if !strings.Contains(envelope.Result.Summary, "--allow-pending requires --reason") {
			t.Fatalf("refusal does not name the remedy: %q", envelope.Result.Summary)
		}
	})

	t.Run("orphan reason is refused", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"release", "check", "--reason", "because", "--json"}, &stdout, &stderr); code == 0 {
			t.Fatal("--reason accepted without --allow-pending")
		}
		envelope := decodeReleaseEnvelope(t, &stdout)
		if !strings.Contains(envelope.Result.Summary, "--allow-pending") {
			t.Fatalf("refusal does not point at --allow-pending: %q", envelope.Result.Summary)
		}
	})

	t.Run("the waiver rides the verdict", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"release", "check", "--allow-pending", "--reason", "deferred to release 1.3", "--json"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("waived pending work still blocked: %s", stdout.String())
		}
		envelope := decodeReleaseEnvelope(t, &stdout)
		if envelope.Result.Completion != "with-waivers" {
			t.Fatalf("completion = %q, want with-waivers", envelope.Result.Completion)
		}
		waiver := envelope.Result.Waiver
		if waiver == nil || waiver.Reason != "deferred to release 1.3" || len(waiver.Tasks) != 2 {
			t.Fatalf("waiver record wrong: %+v", waiver)
		}
		if !strings.Contains(envelope.Result.Summary, "2 task(s) waived (reason: deferred to release 1.3)") {
			t.Fatalf("summary does not name the waiver: %q", envelope.Result.Summary)
		}
	})
}
