package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/selfupdate"
	"github.com/andrearaponi/walden/internal/selfupdate/updatetest"
)

// stubUpdateEnvironment pins the effective version and wires updateOptions
// to an offline release fixture serving latestTag with binaryContent.
func stubUpdateEnvironment(t *testing.T, latestTag, currentVersion string, binaryContent []byte) updatetest.Fixture {
	t.Helper()

	fixture := updatetest.New(t, latestTag, currentVersion, binaryContent)

	restoreVersion := Version
	restoreOptions := updateOptions
	t.Cleanup(func() {
		Version = restoreVersion
		updateOptions = restoreOptions
	})

	Version = currentVersion
	updateOptions = func(string) (selfupdate.Options, error) {
		return fixture.Options, nil
	}
	return fixture
}

func TestRunUpdateDevGuardRefusesBeforeAnyNetworkUse(t *testing.T) {
	restoreVersion := Version
	restoreSeam := buildInfoVersion
	restoreOptions := updateOptions
	t.Cleanup(func() {
		Version = restoreVersion
		buildInfoVersion = restoreSeam
		updateOptions = restoreOptions
	})

	Version = "dev"
	buildInfoVersion = func() string { return "(devel)" }
	updateOptions = func(string) (selfupdate.Options, error) {
		t.Fatal("dev guard must fire before options (and any network) are touched")
		return selfupdate.Options{}, nil
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"update"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("update accepted a dev build")
	}
	if !strings.Contains(stderr.String(), "source build") {
		t.Fatalf("stderr %q does not direct to the source-build workflow", stderr.String())
	}
}

func TestRunUpdateRejectsUnknownFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"update", "--no-verify"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("update accepted an unknown flag")
	}
	if !strings.Contains(stderr.String(), "--no-verify") {
		t.Fatalf("stderr %q does not name the rejected flag", stderr.String())
	}
}

func TestRunUpdateVersionFlagRequiresTag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"update", "--version"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("update accepted --version without a tag")
	}
	if !strings.Contains(stderr.String(), "--version") {
		t.Fatalf("stderr %q does not explain the --version usage", stderr.String())
	}
}

func TestRunUpdateCheckExitsZeroAndWritesNothing(t *testing.T) {
	cases := []struct {
		name      string
		latestTag string
		wantWords string
	}{
		{name: "update available", latestTag: "v0.7.0", wantWords: "v0.7.0"},
		{name: "already current", latestTag: "v0.5.0", wantWords: "up to date"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := stubUpdateEnvironment(t, tc.latestTag, "v0.5.0", []byte("unused"))

			var stdout, stderr bytes.Buffer
			exitCode := Run([]string{"update", "--check"}, &stdout, &stderr)

			if exitCode != 0 {
				t.Fatalf("check mode exited %d, want 0 (stderr: %s)", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantWords) {
				t.Fatalf("check output %q does not mention %q", stdout.String(), tc.wantWords)
			}

			content, err := os.ReadFile(fixture.Executable)
			if err != nil {
				t.Fatalf("read executable: %v", err)
			}
			if string(content) != "OLD-BINARY" {
				t.Fatal("check mode modified the executable")
			}
			entries, _ := os.ReadDir(filepath.Dir(fixture.Executable))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".walden-") {
					t.Fatalf("check mode left artifact %s", entry.Name())
				}
			}
		})
	}
}

func TestRunUpdateCheckEmitsJSONEnvelope(t *testing.T) {
	stubUpdateEnvironment(t, "v0.7.0", "v0.5.0", []byte("unused"))

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"update", "--check", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("check mode exited %d, want 0", exitCode)
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Command != "update" || !envelope.OK {
		t.Fatalf("envelope = command %q ok %t, want update/true", envelope.Command, envelope.OK)
	}

	update := envelope.Result.Update
	if update == nil {
		t.Fatal("envelope carries no update block")
	}
	if update.CurrentVersion != "v0.5.0" || update.TargetVersion != "v0.7.0" || !update.UpdateAvailable || update.Applied {
		t.Fatalf("update block = %+v, want v0.5.0 -> v0.7.0 available, not applied", update)
	}
}

func TestRunUpdateAppliesAndReportsJSON(t *testing.T) {
	newBinary := []byte("#!/bin/sh\necho \"walden v0.7.0 (schema v0beta1)\"\n")
	fixture := stubUpdateEnvironment(t, "v0.7.0", "v0.5.0", newBinary)

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"update", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("update exited %d, want 0 (stderr: %s)", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	update := envelope.Result.Update
	if update == nil || !update.Applied || update.TargetVersion != "v0.7.0" {
		t.Fatalf("update block = %+v, want applied v0.7.0", update)
	}
	if len(envelope.Result.ChangedFiles) == 0 || envelope.Result.ChangedFiles[0] == "" {
		t.Fatalf("changed files = %v, want the executable path", envelope.Result.ChangedFiles)
	}
	if !strings.Contains(envelope.Result.NextAction, "/releases/tag/v0.7.0") {
		t.Fatalf("next action %q does not carry the release notes URL", envelope.Result.NextAction)
	}

	installed, _ := os.ReadFile(fixture.Executable)
	if string(installed) != string(newBinary) {
		t.Fatal("executable does not contain the new release binary")
	}
}

func TestRunUpdateUsageListsCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run([]string{"--help"}, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "update [--check] [--version <tag>] [--json]") {
		t.Fatalf("usage does not list the update command: %s", stdout.String())
	}
}
