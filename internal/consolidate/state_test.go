package consolidate

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func fp(seed string) string { return spec.Fingerprint("requirements.md", seed) }

func approvedFeature(name, fingerprint string) FeatureSnapshot {
	return FeatureSnapshot{Name: name, HasRequirements: true, Status: "approved", ApprovedFingerprint: fingerprint, Fresh: true}
}

func snapshotOf(features ...FeatureSnapshot) Snapshot { return Snapshot{Features: features} }

func consolidatedRecord(entries map[string]RecordFeature) *Record {
	return &Record{Schema: RecordSchema, StartedAt: "2026-10-04T06:00:00Z", Features: entries,
		Checkpoint: &Checkpoint{At: "2026-10-04T07:00:00Z", ViewFingerprint: fp("view")}}
}

var approvedView = ViewStatus{State: ViewApproved, ApprovedFingerprint: fp("view")}

func TestPendingContractChanges(t *testing.T) {
	record := consolidatedRecord(map[string]RecordFeature{
		"alpha": {State: StateConsolidated, RequirementsFingerprint: fp("alpha-1"), ActiveIDs: []string{"R1.AC1"}},
		"beta":  {State: StateConsolidated, RequirementsFingerprint: fp("beta-1"), ActiveIDs: []string{"R1.AC1"}},
	})

	state := Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), approvedFeature("beta", fp("beta-1"))), record, nil, approvedView)
	if state.Tracking != TrackingActive || len(state.Pending) != 0 || state.Threshold != ThresholdNone {
		t.Fatalf("unchanged portfolio: %+v", state)
	}

	want := []PendingChange{{Feature: "beta", Kind: ChangeRevised}, {Feature: "gamma", Kind: ChangeNew}}
	for _, revision := range []string{"beta-2", "beta-3"} {
		state = Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), approvedFeature("beta", fp(revision)), approvedFeature("gamma", fp("gamma-1"))), record, nil, approvedView)
		if !reflect.DeepEqual(state.Pending, want) {
			t.Fatalf("after revision %s pending = %+v, want %+v", revision, state.Pending, want)
		}
	}

	inProgress := FeatureSnapshot{Name: "beta", HasRequirements: true, Status: "in-review"}
	editedAfterApproval := FeatureSnapshot{Name: "alpha", HasRequirements: true, Status: "approved", ApprovedFingerprint: fp("alpha-1"), Fresh: false}
	state = Derive(snapshotOf(editedAfterApproval, inProgress), record, nil, approvedView)
	if len(state.Pending) != 0 {
		t.Fatalf("unapproved or stale requirements counted as pending: %+v", state.Pending)
	}
}

func TestRemovedFeatureCountsAsPending(t *testing.T) {
	record := consolidatedRecord(map[string]RecordFeature{
		"alpha": {State: StateConsolidated, RequirementsFingerprint: fp("alpha-1")},
		"delta": {State: StateConsolidated, RequirementsFingerprint: fp("delta-1")},
	})
	state := Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1"))), record, nil, approvedView)
	if want := []PendingChange{{Feature: "delta", Kind: ChangeRemoved}}; !reflect.DeepEqual(state.Pending, want) {
		t.Fatalf("pending = %+v, want %+v", state.Pending, want)
	}
	state = Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), FeatureSnapshot{Name: "delta"}), record, nil, approvedView)
	if len(state.Pending) != 0 {
		t.Fatalf("an existing directory counted as removed: %+v", state.Pending)
	}
}

func TestUnknownConsolidationRecord(t *testing.T) {
	valid := fp("alpha-1")
	cases := map[string]string{
		"corrupt json":    "{not json",
		"wrong schema":    `{"schema":"consolidation/v9","started_at":"2026-10-04T06:00:00Z","features":{}}`,
		"unknown state":   `{"schema":"consolidation/v1","started_at":"2026-10-04T06:00:00Z","features":{"alpha":{"state":"done","requirements_fingerprint":"` + valid + `","active_ids":[]}}}`,
		"bad fingerprint": `{"schema":"consolidation/v1","started_at":"2026-10-04T06:00:00Z","features":{"alpha":{"state":"backlog","requirements_fingerprint":"sha256:xyz","active_ids":[]}}}`,
		"bad identifier":  `{"schema":"consolidation/v1","started_at":"2026-10-04T06:00:00Z","features":{"alpha":{"state":"backlog","requirements_fingerprint":"` + valid + `","active_ids":["AC1"]}}}`,
		"unknown field":   `{"schema":"consolidation/v1","started_at":"2026-10-04T06:00:00Z","features":{},"extra":1}`,
		"missing start":   `{"schema":"consolidation/v1","features":{}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, RecordPath)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			record, err := LoadRecord(root)
			if err == nil || !strings.Contains(err.Error(), RecordPath) {
				t.Fatalf("LoadRecord error = %v, want one naming %s", err, RecordPath)
			}
			state := Derive(snapshotOf(approvedFeature("alpha", valid), approvedFeature("beta", fp("beta-1")), approvedFeature("gamma", fp("gamma-1"))), record, err, approvedView)
			if state.Tracking != TrackingUnknown || !strings.Contains(state.Problem, RecordPath) || state.Remedy == "" {
				t.Fatalf("state = %+v, want unknown naming the record with a remedy", state)
			}
			if len(state.Pending) != 0 || state.Threshold != ThresholdNone {
				t.Fatalf("an unreadable record produced pending changes or a threshold: %+v", state)
			}
		})
	}

	t.Run("absent record", func(t *testing.T) {
		record, err := LoadRecord(t.TempDir())
		if record != nil || err != nil {
			t.Fatalf("absent record = %+v, %v; want nil, nil", record, err)
		}
	})

	t.Run("deterministic round trip", func(t *testing.T) {
		root := t.TempDir()
		record := consolidatedRecord(map[string]RecordFeature{
			"beta":  {State: StateBacklog, RequirementsFingerprint: fp("beta-1"), ActiveIDs: []string{"R1.AC1", "C1"}},
			"alpha": {State: StateConsolidated, RequirementsFingerprint: valid, ActiveIDs: []string{"R1.AC1"}, ReservedIDs: []string{"R1.AC2"}},
		})
		if err := SaveRecord(root, *record); err != nil {
			t.Fatal(err)
		}
		first, _ := os.ReadFile(filepath.Join(root, RecordPath))
		if err := SaveRecord(root, *record); err != nil {
			t.Fatal(err)
		}
		second, _ := os.ReadFile(filepath.Join(root, RecordPath))
		if !bytes.Equal(first, second) || !bytes.HasSuffix(first, []byte("\n")) {
			t.Fatalf("record bytes are not deterministic:\n%s\n%s", first, second)
		}
		loaded, err := LoadRecord(root)
		if err != nil || !reflect.DeepEqual(loaded, record) {
			t.Fatalf("round trip = %+v, %v; want %+v", loaded, err, record)
		}
	})

	t.Run("record inconsistent with the approved view", func(t *testing.T) {
		record := consolidatedRecord(map[string]RecordFeature{"alpha": {State: StateConsolidated, RequirementsFingerprint: valid}})
		for _, view := range []ViewStatus{{State: ViewApproved, ApprovedFingerprint: fp("another view")}, {State: ViewAbsent}} {
			state := Derive(snapshotOf(approvedFeature("alpha", valid)), record, nil, view)
			if state.Tracking != TrackingUnknown || !strings.Contains(state.Problem, ViewPath) || state.Remedy == "" {
				t.Fatalf("view %+v: state = %+v, want unknown naming %s", view, state, ViewPath)
			}
		}
	})
}

func TestConsolidationThresholds(t *testing.T) {
	started := &Record{Schema: RecordSchema, StartedAt: "2026-10-04T06:00:00Z", Features: map[string]RecordFeature{}}
	names := []string{"a", "b", "c", "d"}
	for count, want := range map[int]string{0: ThresholdNone, 1: ThresholdNone, 2: ThresholdSuggested, 3: ThresholdDue, 4: ThresholdDue} {
		features := []FeatureSnapshot{}
		for _, name := range names[:count] {
			features = append(features, approvedFeature(name, fp(name)))
		}
		if state := Derive(snapshotOf(features...), started, nil, ViewStatus{State: ViewDraft}); state.Threshold != want || len(state.Pending) != count {
			t.Errorf("%d pending: threshold %q, pending %d; want %q", count, state.Threshold, len(state.Pending), want)
		}
	}
	notStarted := Derive(snapshotOf(approvedFeature("a", fp("a")), approvedFeature("b", fp("b")), approvedFeature("c", fp("c")), FeatureSnapshot{Name: "d", HasRequirements: true, Status: "draft"}), nil, nil, ViewStatus{State: ViewAbsent})
	if notStarted.Tracking != TrackingNotStarted || notStarted.Unconsolidated != 3 || notStarted.Threshold != ThresholdNone || len(notStarted.Pending) != 0 {
		t.Fatalf("not started: %+v, want three unconsolidated features and no threshold", notStarted)
	}
}

func TestBacklogStaysUntilConsolidated(t *testing.T) {
	record := &Record{Schema: RecordSchema, StartedAt: "2026-10-04T06:00:00Z", Features: map[string]RecordFeature{
		"alpha": {State: StateBacklog, RequirementsFingerprint: fp("alpha-1")},
		"beta":  {State: StateBacklog, RequirementsFingerprint: fp("beta-1")},
	}}
	state := Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), approvedFeature("beta", fp("beta-1"))), record, nil, ViewStatus{State: ViewDraft})
	if !reflect.DeepEqual(state.Backlog, []string{"alpha", "beta"}) || len(state.Pending) != 0 {
		t.Fatalf("after start: %+v, want two backlog features and nothing pending", state)
	}
	state = Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), approvedFeature("beta", fp("beta-2"))), record, nil, ViewStatus{State: ViewDraft})
	if !reflect.DeepEqual(state.Backlog, []string{"alpha"}) || !reflect.DeepEqual(state.Pending, []PendingChange{{Feature: "beta", Kind: ChangeRevised}}) {
		t.Fatalf("after revising a backlog feature: %+v", state)
	}
	record.Features["alpha"] = RecordFeature{State: StateConsolidated, RequirementsFingerprint: fp("alpha-1")}
	state = Derive(snapshotOf(approvedFeature("alpha", fp("alpha-1")), approvedFeature("beta", fp("beta-1"))), record, nil, ViewStatus{State: ViewDraft})
	if !reflect.DeepEqual(state.Backlog, []string{"beta"}) {
		t.Fatalf("a consolidated feature stayed in the backlog: %+v", state.Backlog)
	}
}
