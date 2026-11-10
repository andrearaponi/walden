package evidence

import (
	"reflect"
	"testing"
)

func TestDeriveStatesAndPrecedence(t *testing.T) {
	current := ChainFingerprints{Requirements: "sha256:req", Design: "sha256:des"}
	baseRecord := func() Record {
		return Record{
			TaskFingerprint:         "sha256:task",
			TaskFingerprintScheme:   "walden/task-definition/v2",
			Execution:               &ExecutionFacts{Origin: "verify", Policy: VerifyPolicy, AssertionResult: ResultPassed, Integrity: "pure", BeforeCodeIdentity: "sha256:code", AfterCodeIdentity: "sha256:code"},
			RequirementsFingerprint: "sha256:req",
			DesignFingerprint:       "sha256:des",
			TasksFingerprint:        "sha256:tasks",
			CodeIdentity:            "sha256:code",
			Result:                  ResultPassed,
		}
	}
	task := LeafTask{ID: "1.1", Completed: true, Fingerprint: "sha256:task"}

	cases := []struct {
		name            string
		mutate          func(*Record)
		task            LeafTask
		recorded        bool
		currentIdentity string
		identityOK      bool
		want            string
	}{
		{
			name: "verified when every binding matches",
			task: task, recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateVerified,
		},
		{
			name: "pending when the task is unchecked",
			task: LeafTask{ID: "1.1", Completed: false}, recorded: true,
			currentIdentity: "sha256:code", identityOK: true,
			want: StatePending,
		},
		{
			name: "unrecorded when completed without an entry",
			task: task, recorded: false, currentIdentity: "sha256:code", identityOK: true,
			want: StateUnrecorded,
		},
		{
			name:   "failed result wins",
			mutate: func(r *Record) { r.Result = ResultFailed },
			task:   task, recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateFailed,
		},
		{
			name:   "failed beats stale-spec",
			mutate: func(r *Record) { r.Result = ResultFailed; r.RequirementsFingerprint = "sha256:old" },
			task:   task, recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateFailed,
		},
		{
			name:   "requirements change is stale-spec",
			mutate: func(r *Record) { r.RequirementsFingerprint = "sha256:old" },
			task:   task, recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateStaleSpec,
		},
		{
			name:   "design change is stale-spec",
			mutate: func(r *Record) { r.DesignFingerprint = "sha256:old" },
			task:   task, recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateStaleSpec,
		},
		{
			name:     "task definition change is stale-spec",
			task:     LeafTask{ID: "1.1", Completed: true, Fingerprint: "sha256:edited"},
			recorded: true, currentIdentity: "sha256:code", identityOK: true,
			want: StateStaleSpec,
		},
		{
			name:   "stale-spec beats stale-code",
			mutate: func(r *Record) { r.RequirementsFingerprint = "sha256:old" },
			task:   task, recorded: true, currentIdentity: "sha256:moved", identityOK: true,
			want: StateStaleSpec,
		},
		{
			name: "code change is stale-code",
			task: task, recorded: true, currentIdentity: "sha256:moved", identityOK: true,
			want: StateStaleCode,
		},
		{
			name:   "absent identities cannot attest equality",
			mutate: func(r *Record) { r.CodeIdentity = "" },
			task:   task, recorded: true, currentIdentity: "", identityOK: false,
			want: StateUnattested,
		},
		{
			name: "recorded identity with absent current is unavailable, not a known change",
			task: task, recorded: true, currentIdentity: "", identityOK: false,
			want: StateUnattested,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := baseRecord()
			if tc.mutate != nil {
				tc.mutate(&record)
			}
			document := Document{Feature: "f", Tasks: map[string]Record{}}
			if tc.recorded {
				document.Tasks["1.1"] = record
			}

			derived := Derive(document, current, tc.currentIdentity, tc.identityOK, []LeafTask{tc.task})
			if len(derived) != 1 {
				t.Fatalf("derived %d entries, want 1", len(derived))
			}
			if derived[0].State != tc.want {
				t.Fatalf("state = %s, want %s", derived[0].State, tc.want)
			}
		})
	}
}

func TestDeriveNeverConsultsTimestamps(t *testing.T) {
	current := ChainFingerprints{Requirements: "sha256:req", Design: "sha256:des"}
	record := Record{
		TaskFingerprint:         "sha256:task",
		TaskFingerprintScheme:   "walden/task-definition/v2",
		Execution:               &ExecutionFacts{Origin: "verify", Policy: VerifyPolicy, AssertionResult: ResultPassed, Integrity: "pure", BeforeCodeIdentity: "sha256:code", AfterCodeIdentity: "sha256:code"},
		RequirementsFingerprint: "sha256:req",
		DesignFingerprint:       "sha256:des",
		CodeIdentity:            "sha256:code",
		Result:                  ResultPassed,
		VerifiedAt:              "1999-01-01T00:00:00Z",
	}
	document := Document{Feature: "f", Tasks: map[string]Record{"1.1": record}}
	tasks := []LeafTask{{ID: "1.1", Completed: true, Fingerprint: "sha256:task"}}

	derived := Derive(document, current, "sha256:code", true, tasks)
	if derived[0].State != StateVerified {
		t.Fatalf("an ancient timestamp influenced the verdict: %s", derived[0].State)
	}
}

// The facts that explain a record sit beside its bindings and judge
// nothing: Derive gives the same assessment with them and without them, in
// every state it can derive.
func TestDeriveIgnoresDiagnosticFacts(t *testing.T) {
	current := ChainFingerprints{Requirements: "sha256:req", Design: "sha256:des"}
	baseRecord := func() Record {
		return Record{
			TaskFingerprint:         "sha256:task",
			TaskFingerprintScheme:   "walden/task-definition/v2",
			Execution:               &ExecutionFacts{Origin: "verify", Policy: VerifyPolicy, AssertionResult: ResultPassed, Integrity: "pure", BeforeCodeIdentity: "sha256:code", AfterCodeIdentity: "sha256:code"},
			RequirementsFingerprint: "sha256:req",
			DesignFingerprint:       "sha256:des",
			TasksFingerprint:        "sha256:tasks",
			CodeIdentity:            "sha256:code",
			Steps:                   []StepResult{{Command: []string{"prove"}}},
			Result:                  ResultPassed,
		}
	}
	task := LeafTask{ID: "1.1", Completed: true, Fingerprint: "sha256:task"}

	cases := []struct {
		state           string
		mutate          func(*Record)
		task            LeafTask
		recorded        bool
		currentIdentity string
		identityOK      bool
	}{
		{state: StateVerified, task: task, recorded: true, currentIdentity: "sha256:code", identityOK: true},
		{state: StatePending, task: LeafTask{ID: "1.1"}, recorded: true, currentIdentity: "sha256:code", identityOK: true},
		{state: StateUnrecorded, task: task, currentIdentity: "sha256:code", identityOK: true},
		{state: StateFailed, mutate: func(r *Record) { r.Result = ResultFailed }, task: task, recorded: true, currentIdentity: "sha256:code", identityOK: true},
		{state: StateStaleSpec, mutate: func(r *Record) { r.RequirementsFingerprint = "sha256:old" }, task: task, recorded: true, currentIdentity: "sha256:code", identityOK: true},
		{state: StateStaleCode, task: task, recorded: true, currentIdentity: "sha256:moved", identityOK: true},
		{state: StateUnattested, mutate: func(r *Record) { r.CodeIdentity = "" }, task: task, recorded: true},
	}

	// Facts of every shape a record can hold, including ones that would
	// explain a change: none may reach the assessment.
	duration := int64(1200)
	factSets := map[string]func(*Record){
		"clean tree": func(r *Record) {
			r.Tree = &TreeFacts{Commit: "1111111111111111111111111111111111111111"}
		},
		"differing paths": func(r *Record) {
			r.Tree = &TreeFacts{Commit: "1111111111111111111111111111111111111111", Paths: map[string]string{"a.go": "100644:aaa", "gone.go": ""}}
		},
		"paths not kept": func(r *Record) {
			r.Tree = &TreeFacts{Commit: "1111111111111111111111111111111111111111", PathsOmitted: 500}
		},
		"statements": func(r *Record) {
			r.Statements = &StatementFacts{Scheme: "walden/statement-block/v1", Fingerprints: map[string]string{"R1.AC1": "sha256:a"}}
		},
		"unknown scheme": func(r *Record) {
			r.Statements = &StatementFacts{Scheme: "walden/statement-block/v9", Fingerprints: map[string]string{}}
		},
		"durations": func(r *Record) {
			for i := range r.Steps {
				r.Steps[i].DurationMS = &duration
			}
		},
		"everything": func(r *Record) {
			r.Tree = &TreeFacts{Commit: "not even an object id", Paths: map[string]string{"a.go": "100644:aaa"}}
			r.Statements = &StatementFacts{Scheme: "walden/statement-block/v1", Fingerprints: map[string]string{"R1.AC1": "sha256:a"}}
			for i := range r.Steps {
				r.Steps[i].DurationMS = &duration
			}
		},
	}

	reached := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			plain := baseRecord()
			if tc.mutate != nil {
				tc.mutate(&plain)
			}
			derive := func(record Record) TaskEvidence {
				document := Document{Feature: "f", Tasks: map[string]Record{}}
				if tc.recorded {
					document.Tasks["1.1"] = record
				}
				derived := Derive(document, current, tc.currentIdentity, tc.identityOK, []LeafTask{tc.task})
				if len(derived) != 1 {
					t.Fatalf("derived %d entries, want 1", len(derived))
				}
				return derived[0]
			}

			without := derive(plain)
			if without.State != tc.state {
				t.Fatalf("the fixture derives %s, want %s", without.State, tc.state)
			}
			reached[without.State] = true

			for name, add := range factSets {
				with := baseRecord()
				if tc.mutate != nil {
					tc.mutate(&with)
				}
				add(&with)
				if got := derive(with); !reflect.DeepEqual(got, without) {
					t.Fatalf("%s changed the assessment:\nwith    %+v\nwithout %+v", name, got, without)
				}
			}
		})
	}

	for _, state := range []string{StateVerified, StateFailed, StateStaleSpec, StateStaleCode, StateUnattested, StateUnrecorded, StatePending} {
		if !reached[state] {
			t.Fatalf("state %s was not exercised", state)
		}
	}
}
