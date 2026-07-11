package evidence

import "testing"

func TestDeriveStatesAndPrecedence(t *testing.T) {
	current := ChainFingerprints{Requirements: "sha256:req", Design: "sha256:des"}
	baseRecord := func() Record {
		return Record{
			TaskFingerprint:         "sha256:task",
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
			name:   "absent identities compare equal",
			mutate: func(r *Record) { r.CodeIdentity = "" },
			task:   task, recorded: true, currentIdentity: "", identityOK: false,
			want: StateVerified,
		},
		{
			name: "recorded identity with absent current is stale-code",
			task: task, recorded: true, currentIdentity: "", identityOK: false,
			want: StateStaleCode,
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
