package ears

import (
	"slices"
	"strings"
	"testing"
)

// TestKeywordRecognition pins when a condition keyword opens a clause: at the
// start of the criterion or after a comma in any letter case, or in capitals
// anywhere before SHALL. Any other occurrence is text of its clause.
func TestKeywordRecognition(t *testing.T) {
	tests := []struct {
		name string
		text string
		form string
	}{
		{"if inside an event clause", "WHEN the user asks if the file exists, the system SHALL answer with its path", FormEventDriven},
		{"where inside an event clause", "WHEN the user opens the page where the list is shown, the system SHALL load the list", FormEventDriven},
		{"while inside an event clause", "WHEN a download completes while offline, the system SHALL queue the upload", FormEventDriven},
		{"during inside an event clause", "WHEN a proof finishes during a verify run, the system SHALL record its outcome", FormEventDriven},
		{"when inside a state clause", "WHILE waiting between cycles or when idle, the system SHALL show the idle state", FormStateDriven},
		{"if inside a state clause", "WHILE a client checks if the session is alive, the system SHALL keep the socket open", FormStateDriven},
		{"mixed case inside a clause", "WHEN the job ends While the queue drains, the system SHALL report the job", FormEventDriven},
		{"if in a header name", "WHEN a request carries If-Modified-Since, the system SHALL compare the dates", FormEventDriven},
		{"doubled keyword is one word", "The system SHALL log the WHENWHEN marker", FormUbiquitous},
		{"sentence case at the start", "When the user saves, the system SHALL store the draft", FormEventDriven},
		{"lowercase at the start", "if the upload fails, then the system shall keep the local copy", FormUnwanted},
		{"lowercase after a comma", "WHILE a turn runs, when the container exits, the system SHALL report the exit", FormComplex},
		{"capitals inside a clause", "WHEN the user submits an instruction WHILE a selection exists, the system SHALL apply the instruction", FormComplex},
		{"capitals after THEN", "IF the first check fails, THEN WHEN the operator retries, the system SHALL run the full check", FormComplex},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCriterion("R1.AC1", tc.text)
			if !got.Valid {
				t.Fatalf("want valid %s, got errors %v", tc.form, got.Errors)
			}
			if got.Form != tc.form {
				t.Fatalf("want form %s, got %s", tc.form, got.Form)
			}
		})
	}
}

// TestClauseCombinations pins the form of every combination of clause kinds
// and the clause kinds a valid criterion reports.
func TestClauseCombinations(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		form    string
		clauses []string
	}{
		{"optional and state", "WHERE push is configured, WHILE the app is in background, the system SHALL deliver notifications", FormComplex, []string{ClauseOptional, ClauseState}},
		{"optional and event", "WHERE voice replies are enabled, WHEN a turn ends, the system SHALL speak the reply", FormComplex, []string{ClauseOptional, ClauseEvent}},
		{"optional and unwanted", "WHERE quotas apply, IF a quota is exceeded, THEN the system SHALL reject the request", FormComplex, []string{ClauseOptional, ClauseUnwanted}},
		{"state and event", "WHILE a sync runs, WHEN the user saves, the system SHALL queue the draft", FormComplex, []string{ClauseState, ClauseEvent}},
		{"state and unwanted", "WHILE a turn is running, IF the container exits, THEN the system SHALL report the exit", FormComplex, []string{ClauseState, ClauseUnwanted}},
		{"event and unwanted", "WHEN the daemon starts, IF sub-agents are recorded as running, THEN the system SHALL mark them interrupted", FormComplex, []string{ClauseEvent, ClauseUnwanted}},
		{"optional state and event", "WHERE backups are enabled, WHILE a backup runs, WHEN the disk fills, the system SHALL pause the backup", FormComplex, []string{ClauseOptional, ClauseState, ClauseEvent}},
		{"optional state and unwanted", "WHERE backups are enabled, WHILE a backup runs, IF the disk fills, THEN the system SHALL pause the backup", FormComplex, []string{ClauseOptional, ClauseState, ClauseUnwanted}},
		{"during and event", "DURING an import, WHEN a record fails, the system SHALL log the record", FormComplex, []string{ClauseState, ClauseEvent}},
		{"two event clauses", "WHEN the token is expired, WHEN a refresh arrives, the system SHALL issue a new token", FormEventDriven, []string{ClauseEvent, ClauseEvent}},
		{"two state clauses", "WHILE online, WHILE the queue is empty, the system SHALL sleep", FormStateDriven, []string{ClauseState, ClauseState}},
		{"lone optional clause", "WHERE offline mode is enabled, the system SHALL queue edits", FormOptional, []string{ClauseOptional}},
		{"lone unwanted clause", "IF the upload fails, THEN the system SHALL keep the local copy", FormUnwanted, []string{ClauseUnwanted}},
		{"no clause", "The system SHALL keep one draft per document", FormUbiquitous, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCriterion("R1.AC1", tc.text)
			if !got.Valid {
				t.Fatalf("want valid %s, got errors %v", tc.form, got.Errors)
			}
			if got.Form != tc.form {
				t.Fatalf("want form %s, got %s", tc.form, got.Form)
			}
			if !slices.Equal(got.Clauses, tc.clauses) {
				t.Fatalf("want clauses %v, got %v", tc.clauses, got.Clauses)
			}
		})
	}
}

// TestClauseErrors pins the structural errors: an unwanted-behavior clause
// without THEN and an empty clause anywhere, and the errors on SHALL and the
// response, unchanged.
func TestClauseErrors(t *testing.T) {
	tests := []struct {
		name string
		text string
		err  string
	}{
		{"missing THEN in a complex criterion", "WHILE a turn is running, IF the container exits, the system SHALL report the exit", "IF keyword requires matching THEN before SHALL"},
		{"missing THEN in a lone unwanted clause", "IF the upload fails, the system SHALL retry", "IF keyword requires matching THEN before SHALL"},
		{"empty optional clause in a complex criterion", "WHERE , WHEN a turn ends, the system SHALL speak the reply", "empty feature slot after WHERE"},
		{"empty state clause in a complex criterion", "WHILE , WHEN a turn ends, the system SHALL speak the reply", "empty precondition slot after WHILE"},
		{"empty during clause in a complex criterion", "DURING , WHEN a record fails, the system SHALL log the record", "empty precondition slot after DURING"},
		{"empty event clause in a complex criterion", "WHERE voice replies are enabled, WHEN , the system SHALL speak the reply", "empty trigger slot after WHEN"},
		{"empty unwanted clause in a complex criterion", "WHILE a turn runs, IF , THEN the system SHALL report the exit", "empty trigger slot between IF and THEN"},
		{"repeated clause left empty", "WHEN the user saves, WHEN , the system SHALL store the draft", "empty trigger slot after WHEN"},
		{"missing SHALL", "WHEN the user saves, the system stores the draft", "missing required keyword SHALL"},
		{"two SHALL", "The system SHALL save the draft and SHALL notify the owner", "criterion contains 2 occurrences of SHALL"},
		{"empty response", "WHEN the user saves, the system SHALL", "empty response slot after SHALL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCriterion("R1.AC1", tc.text)
			if got.Valid {
				t.Fatalf("want invalid, got valid %s", got.Form)
			}
			if len(got.Errors) != 1 || !strings.Contains(got.Errors[0], tc.err) {
				t.Fatalf("want one error containing %q, got %v", tc.err, got.Errors)
			}
		})
	}

	// THEN closes an IF clause in any letter case, with or without a comma.
	for _, text := range []string{
		"if the upload fails, then the system shall keep the local copy",
		"IF the upload fails THEN the system SHALL keep the local copy",
	} {
		if got := ParseCriterion("R1.AC1", text); !got.Valid || got.Form != FormUnwanted {
			t.Errorf("%q: want valid unwanted, got valid=%v form=%q errors=%v", text, got.Valid, got.Form, got.Errors)
		}
	}
}

// TestOrderWarning pins the warning for clauses out of the EARS order and
// that it changes neither validity nor form.
func TestOrderWarning(t *testing.T) {
	tests := []struct {
		name       string
		outOfOrder string
		inOrder    string
	}{
		{
			"event before state",
			"WHEN a message arrives, WHILE the user is offline, the system SHALL store the message",
			"WHILE the user is offline, WHEN a message arrives, the system SHALL store the message",
		},
		{
			"state before optional feature",
			"WHILE a backup runs, WHERE backups are enabled, the system SHALL show the backup state",
			"WHERE backups are enabled, WHILE a backup runs, the system SHALL show the backup state",
		},
		{
			"event before optional feature",
			"WHEN a turn ends, WHERE voice replies are enabled, the system SHALL speak the reply",
			"WHERE voice replies are enabled, WHEN a turn ends, the system SHALL speak the reply",
		},
		{
			"unwanted behavior before state",
			"IF the disk fills, THEN WHILE a backup runs, the system SHALL pause the backup",
			"WHILE a backup runs, IF the disk fills, THEN the system SHALL pause the backup",
		},
		{
			"capitalized state keyword inside an event clause",
			"WHEN the user submits an instruction WHILE a selection exists, the system SHALL apply the instruction",
			"WHILE a selection exists, WHEN the user submits an instruction, the system SHALL apply the instruction",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCriterion("R1.AC1", tc.outOfOrder)
			want := ParseCriterion("R1.AC1", tc.inOrder)
			if !got.Valid || !want.Valid {
				t.Fatalf("want both valid, got %v %v and %v %v", got.Valid, got.Errors, want.Valid, want.Errors)
			}
			if got.Form != want.Form {
				t.Fatalf("out of order gives %s, in order %s", got.Form, want.Form)
			}
			if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "WHERE → WHILE → WHEN/IF") {
				t.Fatalf("want one warning naming WHERE → WHILE → WHEN/IF, got %v", got.Warnings)
			}
			if len(want.Warnings) != 0 {
				t.Fatalf("in order: want no warning, got %v", want.Warnings)
			}
		})
	}

	// An event clause and an unwanted-behavior clause share a place in the
	// order, in either sequence.
	for _, text := range []string{
		"WHEN the daemon starts, IF sub-agents are running, THEN the system SHALL mark the sub-agents interrupted",
		"IF the first check fails, THEN WHEN the operator retries, the system SHALL run the full check",
	} {
		if got := ParseCriterion("R1.AC1", text); !got.Valid || len(got.Warnings) != 0 {
			t.Errorf("%q: want valid without warnings, got valid=%v warnings=%v errors=%v", text, got.Valid, got.Warnings, got.Errors)
		}
	}
}

// TestSubjectWarning pins the warning for a pronoun subject and that it
// changes neither validity nor form.
func TestSubjectWarning(t *testing.T) {
	tests := []struct {
		name    string
		pronoun string
		text    string
		named   string
	}{
		{
			"it after a comma", "it",
			"WHEN the system assembles the export, it SHALL order the sections",
			"WHEN the system assembles the export, the exporter SHALL order the sections",
		},
		{
			"they after THEN", "they",
			"IF the export fails, THEN they SHALL keep the previous file",
			"IF the export fails, THEN the exporters SHALL keep the previous file",
		},
		{
			"it after THEN without a comma", "it",
			"IF the export fails THEN it SHALL keep the previous file",
			"IF the export fails THEN the exporter SHALL keep the previous file",
		},
		{
			"It opening a criterion without clauses", "It",
			"It SHALL keep one export per day",
			"The exporter SHALL keep one export per day",
		},
		{
			"IT after a comma", "IT",
			"WHEN the projection service is built, IT SHALL expose no write operation",
			"WHEN the projection service is built, the service SHALL expose no write operation",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCriterion("R1.AC1", tc.text)
			want := ParseCriterion("R1.AC1", tc.named)
			if !got.Valid || !want.Valid {
				t.Fatalf("want both valid, got %v %v and %v %v", got.Valid, got.Errors, want.Valid, want.Errors)
			}
			if got.Form != want.Form {
				t.Fatalf("pronoun subject gives %s, named subject %s", got.Form, want.Form)
			}
			quoted := `subject "` + tc.pronoun + `" is a pronoun`
			if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], quoted) {
				t.Fatalf("want one warning containing %q, got %v", quoted, got.Warnings)
			}
			if len(want.Warnings) != 0 {
				t.Fatalf("named subject: want no warning, got %v", want.Warnings)
			}
		})
	}

	for _, text := range []string{
		"WHEN an order ships, items SHALL leave the stock",
		"WHEN the export runs it SHALL log the start",
	} {
		if got := ParseCriterion("R1.AC1", text); !got.Valid || len(got.Warnings) != 0 {
			t.Errorf("%q: want valid without warnings, got valid=%v warnings=%v errors=%v", text, got.Valid, got.Warnings, got.Errors)
		}
	}
}
