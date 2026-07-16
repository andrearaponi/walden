package evidence

// Derived task states, ordered by precedence: failed > stale-spec >
// stale-code > verified; unrecorded and pending stand apart.
const (
	StateVerified   = "verified"
	StateFailed     = "failed"
	StateStaleSpec  = "stale-spec"
	StateStaleCode  = "stale-code"
	StateUnrecorded = "unrecorded"
	StatePending    = "pending"
)

// ChainFingerprints carries the current approved fingerprints records are
// compared against. Task-definition fingerprints travel on each LeafTask.
type ChainFingerprints struct {
	Requirements string
	Design       string
}

// LeafTask is the minimal task view derivation needs.
type LeafTask struct {
	ID          string
	Completed   bool
	Fingerprint string
}

// TaskEvidence is one task's derived execution state.
type TaskEvidence struct {
	TaskID           string
	State            string
	RecordedIdentity string
	CurrentIdentity  string
	// Profile context is filled by presentation layers, never by Derive:
	// the profile diagnoses, it does not judge.
	RecordedProfile Profile
	ProfileDrift    []ProfileDrift
	ProfileLegacy   bool
}

// Derive computes every task's state from facts at read time — the record's
// bindings against the current chain and code identity. Nothing here reads
// or writes stored status labels, and timestamps never participate.
func Derive(document Document, current ChainFingerprints, currentIdentity string, identityOK bool, tasks []LeafTask) []TaskEvidence {
	derived := make([]TaskEvidence, 0, len(tasks))
	if !identityOK {
		currentIdentity = ""
	}

	for _, task := range tasks {
		record, recorded := document.Tasks[task.ID]

		entry := TaskEvidence{TaskID: task.ID, CurrentIdentity: currentIdentity}
		switch {
		case !task.Completed:
			entry.State = StatePending
		case !recorded:
			entry.State = StateUnrecorded
		default:
			entry.RecordedIdentity = record.CodeIdentity
			entry.State = recordState(record, current, task, currentIdentity)
		}
		derived = append(derived, entry)
	}
	return derived
}

func recordState(record Record, current ChainFingerprints, task LeafTask, currentIdentity string) string {
	switch {
	case record.Result != ResultPassed:
		return StateFailed
	case record.RequirementsFingerprint != current.Requirements,
		record.DesignFingerprint != current.Design,
		record.TaskFingerprint != task.Fingerprint:
		return StateStaleSpec
	case record.CodeIdentity != currentIdentity:
		// Two absent identities compare equal: no-git repositories never
		// drown in stale-code noise.
		return StateStaleCode
	default:
		return StateVerified
	}
}
