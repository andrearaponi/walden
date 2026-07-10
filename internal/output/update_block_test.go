package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestUpdateBlockMarshalsWhenSet(t *testing.T) {
	var buffer bytes.Buffer
	result := Result{
		Summary: "update check",
		Update: &UpdateStatus{
			CurrentVersion:  "v0.5.0",
			TargetVersion:   "v0.7.0",
			UpdateAvailable: true,
			Applied:         false,
		},
		ExitCode: 0,
	}

	if err := PrintJSON(&buffer, "update", result); err != nil {
		t.Fatalf("PrintJSON returned error: %v", err)
	}

	var envelope struct {
		Command string `json:"command"`
		Result  struct {
			Update map[string]any `json:"update"`
		} `json:"result"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Command != "update" {
		t.Fatalf("command = %q, want update", envelope.Command)
	}

	update := envelope.Result.Update
	if update["current_version"] != "v0.5.0" || update["target_version"] != "v0.7.0" {
		t.Fatalf("update block = %v, want current_version/target_version populated", update)
	}
	if update["update_available"] != true || update["applied"] != false {
		t.Fatalf("update block = %v, want update_available=true applied=false", update)
	}
}

func TestUpdateBlockOmittedWhenAbsent(t *testing.T) {
	var buffer bytes.Buffer
	if err := PrintJSON(&buffer, "status", Result{Summary: "x", ExitCode: 0}); err != nil {
		t.Fatalf("PrintJSON returned error: %v", err)
	}
	if strings.Contains(buffer.String(), "\"update\"") {
		t.Fatalf("update block leaked into an envelope that did not set it: %s", buffer.String())
	}
}

func TestUpdateBlockRendersInText(t *testing.T) {
	var buffer bytes.Buffer
	PrintText(&buffer, Result{
		Summary: "update check",
		Update: &UpdateStatus{
			CurrentVersion:  "v0.5.0",
			TargetVersion:   "v0.7.0",
			UpdateAvailable: true,
		},
	})

	out := buffer.String()
	if !strings.Contains(out, "v0.5.0") || !strings.Contains(out, "v0.7.0") {
		t.Fatalf("text output does not render the update versions: %q", out)
	}
}
