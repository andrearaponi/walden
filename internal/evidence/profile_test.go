package evidence

import (
	"strings"
	"testing"
)

func TestProfileDiff(t *testing.T) {
	recorded := Profile{"platform": "darwin/arm64", "go": "go1.25.0", "gone": "was-here"}
	current := Profile{"platform": "darwin/arm64", "go": "go1.24.0", "new": "appeared"}

	drifts := DiffProfile(recorded, current)
	keys := []string{}
	for _, drift := range drifts {
		keys = append(keys, drift.Key)
	}
	if strings.Join(keys, ",") != "go,gone,new" {
		t.Fatalf("unexpected drift keys (must be sorted): %v", keys)
	}
	if drifts[0].Recorded != "go1.25.0" || drifts[0].Current != "go1.24.0" {
		t.Fatalf("value drift not captured: %+v", drifts[0])
	}
	if drifts[1].Current != "" || drifts[2].Recorded != "" {
		t.Fatalf("added/removed keys must carry an empty side: %+v", drifts)
	}

	if diff := DiffProfile(recorded, recorded); len(diff) != 0 {
		t.Fatalf("identical profiles must not drift: %v", diff)
	}
	if diff := DiffProfile(nil, current); diff != nil {
		t.Fatalf("legacy (nil recorded) must short-circuit: %v", diff)
	}
	if diff := DiffProfile(recorded, nil); diff != nil {
		t.Fatalf("nil current must short-circuit: %v", diff)
	}
}

func TestDeriveIgnoresProfile(t *testing.T) {
	fingerprint := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	identity := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	current := ChainFingerprints{Requirements: "sha256:aaa", Design: "sha256:bbb"}
	record := Record{
		TaskFingerprint:         fingerprint,
		RequirementsFingerprint: current.Requirements,
		DesignFingerprint:       current.Design,
		CodeIdentity:            identity,
		Result:                  ResultPassed,
	}
	withProfile := record
	withProfile.Profile = Profile{"platform": "linux/amd64", "go": "go1.25.0"}

	leafs := []LeafTask{{ID: "1.1", Completed: true, Fingerprint: fingerprint}}
	plain := Derive(Document{Tasks: map[string]Record{"1.1": record}}, current, identity, true, leafs)
	profiled := Derive(Document{Tasks: map[string]Record{"1.1": withProfile}}, current, identity, true, leafs)

	if plain[0].State != profiled[0].State {
		t.Fatalf("profile presence changed the derived state: %s vs %s", plain[0].State, profiled[0].State)
	}
	if plain[0].State != StateVerified {
		t.Fatalf("fixture expected verified, got %s", plain[0].State)
	}

	failed := withProfile
	failed.Result = ResultFailed
	failedPlain := record
	failedPlain.Result = ResultFailed
	a := Derive(Document{Tasks: map[string]Record{"1.1": failedPlain}}, current, identity, true, leafs)
	b := Derive(Document{Tasks: map[string]Record{"1.1": failed}}, current, identity, true, leafs)
	if a[0].State != b[0].State || a[0].State != StateFailed {
		t.Fatalf("failed-state parity broken: %s vs %s", a[0].State, b[0].State)
	}
}
