package ears

import (
	"strings"
	"testing"
)

func TestParseCriterionClassifiesAllSixForms(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantForm string
		wantErr  bool
	}{
		{
			name:     "ubiquitous",
			text:     "The system SHALL prevent engine overspeed",
			wantForm: FormUbiquitous,
		},
		{
			name:     "event-driven",
			text:     "WHEN the user submits a todo, the system SHALL create it",
			wantForm: FormEventDriven,
		},
		{
			name:     "state-driven",
			text:     "WHILE the engine is running, the system SHALL monitor temperature",
			wantForm: FormStateDriven,
		},
		{
			name:     "optional",
			text:     "WHERE the premium feature is enabled, the system SHALL show analytics",
			wantForm: FormOptional,
		},
		{
			name:     "unwanted",
			text:     "IF the database connection fails, THEN the system SHALL retry with backoff",
			wantForm: FormUnwanted,
		},
		{
			name:     "complex",
			text:     "WHILE the engine is running, WHEN continuous ignition is commanded, the system SHALL provide continuous ignition",
			wantForm: FormComplex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseCriterion("R1.AC1", tc.text)
			if !result.Valid {
				t.Fatalf("expected valid, got errors: %v", result.Errors)
			}
			if result.Form != tc.wantForm {
				t.Fatalf("expected form %q, got %q", tc.wantForm, result.Form)
			}
		})
	}
}

func TestParseCriterionRejectsMissingSHALL(t *testing.T) {
	result := ParseCriterion("R1.AC1", "WHEN the user clicks, the system creates a todo")
	if result.Valid {
		t.Fatal("expected invalid for missing SHALL")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "missing required keyword SHALL") {
		t.Fatalf("expected SHALL error, got: %v", result.Errors)
	}
}

func TestParseCriterionRejectsIFWithoutTHEN(t *testing.T) {
	result := ParseCriterion("R1.AC1", "IF the connection fails, the system SHALL retry")
	if result.Valid {
		t.Fatal("expected invalid for IF without THEN")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "IF keyword requires matching THEN") {
		t.Fatalf("expected IF/THEN error, got: %v", result.Errors)
	}
}

func TestParseCriterionHandlesCaseInsensitiveKeywords(t *testing.T) {
	result := ParseCriterion("R1.AC1", "When the user submits a todo, the system shall create it")
	if !result.Valid {
		t.Fatalf("expected case-insensitive match to be valid, got errors: %v", result.Errors)
	}
	if result.Form != FormEventDriven {
		t.Fatalf("expected event-driven form, got %q", result.Form)
	}
}

func TestParseCriterionDoesNotMatchPartialKeywords(t *testing.T) {
	result := ParseCriterion("R1.AC1", "The system SHALL update the WHENEVER counter")
	if !result.Valid {
		t.Fatalf("expected valid ubiquitous, got errors: %v", result.Errors)
	}
	if result.Form != FormUbiquitous {
		t.Fatalf("expected ubiquitous form (WHENEVER should not match WHEN), got %q", result.Form)
	}
}

func TestParseAllCriteriaExtractsFromBody(t *testing.T) {
	body := `## Requirements

### R1 Feature

#### Acceptance Criteria

1. ` + "`R1.AC1`" + ` WHEN triggered, the system SHALL respond
2. ` + "`R1.AC2`" + ` The system SHALL remain stable
3. ` + "`R1.AC3`" + ` IF failure occurs, THEN the system SHALL recover

### R2 Other

#### Acceptance Criteria

1. ` + "`R2.AC1`" + ` WHILE active, WHEN triggered, the system SHALL react
`

	results := ParseAllCriteria(body)
	if len(results) != 4 {
		t.Fatalf("expected 4 criteria, got %d", len(results))
	}

	expected := []struct {
		id   string
		form string
	}{
		{"R1.AC1", FormEventDriven},
		{"R1.AC2", FormUbiquitous},
		{"R1.AC3", FormUnwanted},
		{"R2.AC1", FormComplex},
	}

	for i, exp := range expected {
		if results[i].ID != exp.id {
			t.Fatalf("result %d: expected ID %q, got %q", i, exp.id, results[i].ID)
		}
		if !results[i].Valid {
			t.Fatalf("result %d (%s): expected valid, got errors: %v", i, exp.id, results[i].Errors)
		}
		if results[i].Form != exp.form {
			t.Fatalf("result %d (%s): expected form %q, got %q", i, exp.id, exp.form, results[i].Form)
		}
	}
}

func TestSlotEmptyTriggerRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "WHEN , the system SHALL respond")
	if result.Valid {
		t.Fatal("expected invalid for empty trigger slot")
	}
	if !strings.Contains(result.Errors[0], "empty trigger slot") {
		t.Fatalf("expected trigger slot error, got: %v", result.Errors)
	}
}

func TestSlotEmptyPreconditionRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "WHILE , the system SHALL respond")
	if result.Valid {
		t.Fatal("expected invalid for empty precondition slot")
	}
	if !strings.Contains(result.Errors[0], "empty precondition slot") {
		t.Fatalf("expected precondition slot error, got: %v", result.Errors)
	}
}

func TestSlotEmptyResponseRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "WHEN something happens, the system SHALL")
	if result.Valid {
		t.Fatal("expected invalid for empty response slot")
	}
	if !strings.Contains(result.Errors[0], "empty response slot") {
		t.Fatalf("expected response slot error, got: %v", result.Errors)
	}
}

func TestSlotEmptyResponseUbiquitousRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "The system SHALL")
	if result.Valid {
		t.Fatal("expected invalid for empty response in ubiquitous form")
	}
	if !strings.Contains(result.Errors[0], "empty response slot") {
		t.Fatalf("expected response slot error, got: %v", result.Errors)
	}
}

func TestSlotValidNonEmptyPasses(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"event-driven", "WHEN the user clicks, the system SHALL respond"},
		{"state-driven", "WHILE running, the system SHALL monitor"},
		{"optional", "WHERE premium, the system SHALL show analytics"},
		{"unwanted", "IF failure, THEN the system SHALL recover"},
		{"complex", "WHILE running, WHEN triggered, the system SHALL react"},
		{"ubiquitous", "The system SHALL work"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseCriterion("R1.AC1", tc.text)
			if !result.Valid {
				t.Fatalf("expected valid, got errors: %v", result.Errors)
			}
		})
	}
}

func TestSlotEmptyIfTriggerRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "IF , THEN the system SHALL respond")
	if result.Valid {
		t.Fatal("expected invalid for empty IF trigger slot")
	}
	if !strings.Contains(result.Errors[0], "empty trigger slot between IF and THEN") {
		t.Fatalf("expected IF trigger slot error, got: %v", result.Errors)
	}
}

func TestDuringClassifiedAsStateDriven(t *testing.T) {
	result := ParseCriterion("R1.AC1", "DURING the flight, the system SHALL monitor altitude")
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.Form != FormStateDriven {
		t.Fatalf("expected state-driven form for DURING, got %q", result.Form)
	}
}

func TestDuringWithWhenClassifiedAsComplex(t *testing.T) {
	result := ParseCriterion("R1.AC1", "DURING the flight, WHEN turbulence is detected, the system SHALL alert")
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.Form != FormComplex {
		t.Fatalf("expected complex form for DURING+WHEN, got %q", result.Form)
	}
}

func TestPostShallKeywordWarningOnUbiquitous(t *testing.T) {
	result := ParseCriterion("R1.AC1", "The system SHALL respond WHEN triggered")
	if !result.Valid {
		t.Fatalf("expected valid (warning, not error), got errors: %v", result.Errors)
	}
	if result.Form != FormUbiquitous {
		t.Fatalf("expected ubiquitous form, got %q", result.Form)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected post-SHALL keyword warning")
	}
	if !strings.Contains(result.Warnings[0], "WHEN appears after SHALL") {
		t.Fatalf("expected WHEN warning, got: %v", result.Warnings)
	}
}

func TestPostShallNoWarningOnEventDriven(t *testing.T) {
	result := ParseCriterion("R1.AC1", "WHEN triggered, the system SHALL respond")
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for correctly classified event-driven, got: %v", result.Warnings)
	}
}

func TestMultipleShallRejected(t *testing.T) {
	result := ParseCriterion("R1.AC1", "The system SHALL do X and SHALL do Y")
	if result.Valid {
		t.Fatal("expected invalid for multiple SHALL")
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "2 occurrences of SHALL") {
		t.Fatalf("expected multiple SHALL error, got: %v", result.Errors)
	}
}

func TestSingleShallPasses(t *testing.T) {
	result := ParseCriterion("R1.AC1", "The system SHALL do something")
	if !result.Valid {
		t.Fatalf("expected single SHALL to be valid, got errors: %v", result.Errors)
	}
}

func TestParseCriterionEmptyText(t *testing.T) {
	result := ParseCriterion("R1.AC1", "")
	if result.Valid {
		t.Fatal("expected empty text to be invalid")
	}
}
