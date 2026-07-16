package app

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/workflow"
)

func TestVerifyEnvelopeCarriesWarnings(t *testing.T) {
	result := verifyOutputResult(workflow.VerifyResult{
		Feature:  "demo",
		Warnings: []string{"proof side effects modified the repository: generated.txt"},
	})

	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "proof side effects modified the repository") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the purity warning in the envelope result, got %v", result.Warnings)
	}
}
