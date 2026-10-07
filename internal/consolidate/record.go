package consolidate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/andrearaponi/walden/internal/spec"
)

// RecordPath is the CLI-owned consolidation record, relative to the repository root.
const RecordPath = ".walden/consolidation.json"

// RecordSchema identifies the record format.
const RecordSchema = "consolidation/v1"

// Record states of a feature.
const (
	StateBacklog      = "backlog"
	StateConsolidated = "consolidated"
)

var recordIdentifier = regexp.MustCompile(`^(?:` + spec.CriterionIDExpr + `|` + spec.NFRIDExpr + `|` + spec.ConstraintIDExpr + `)$`)

// Checkpoint binds the last approved consolidation to its view.
type Checkpoint struct {
	At              string `json:"at"`
	ViewFingerprint string `json:"view_fingerprint"`
}

// RecordFeature is one feature's recorded requirements version and identifiers.
type RecordFeature struct {
	State                   string            `json:"state"`
	RequirementsFingerprint string            `json:"requirements_fingerprint"`
	ActiveIDs               []string          `json:"active_ids"`
	ReservedIDs             []string          `json:"reserved_ids,omitempty"`
	Statements              map[string]string `json:"statements,omitempty"`
}

// Record is the consolidation record. Only the CLI writes it.
type Record struct {
	Schema     string                   `json:"schema"`
	StartedAt  string                   `json:"started_at"`
	Checkpoint *Checkpoint              `json:"checkpoint,omitempty"`
	Features   map[string]RecordFeature `json:"features"`
}

// LoadRecord reads and validates the record. An absent record yields nil,
// nil: tracking has not started. Every other failure names the record path.
func LoadRecord(root string) (*Record, error) {
	data, err := os.ReadFile(filepath.Join(root, RecordPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", RecordPath, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("%s is malformed: %v", RecordPath, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s is malformed: unexpected content after the record", RecordPath)
	}
	if err := record.validate(); err != nil {
		return nil, fmt.Errorf("%s is malformed: %v", RecordPath, err)
	}
	return &record, nil
}

// SaveRecord validates the record and writes it atomically with stable bytes.
func SaveRecord(root string, record Record) error {
	if err := record.validate(); err != nil {
		return fmt.Errorf("refusing to write %s: %v", RecordPath, err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", RecordPath, err)
	}
	path := filepath.Join(root, RecordPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(RecordPath), err)
	}
	return spec.WriteFileAtomic(path, append(data, '\n'))
}

func (r *Record) validate() error {
	if r.Schema != RecordSchema {
		return fmt.Errorf("unsupported schema %q, want %q", r.Schema, RecordSchema)
	}
	if _, err := time.Parse(time.RFC3339, r.StartedAt); err != nil {
		return fmt.Errorf("started_at %q is not an RFC 3339 time", r.StartedAt)
	}
	if r.Checkpoint != nil {
		if _, err := time.Parse(time.RFC3339, r.Checkpoint.At); err != nil {
			return fmt.Errorf("checkpoint time %q is not an RFC 3339 time", r.Checkpoint.At)
		}
		if !spec.ValidFingerprint(r.Checkpoint.ViewFingerprint) {
			return fmt.Errorf("checkpoint view fingerprint %q is malformed", r.Checkpoint.ViewFingerprint)
		}
	}
	if r.Features == nil {
		r.Features = map[string]RecordFeature{}
	}
	for name, feature := range r.Features {
		if normalized, err := spec.NormalizeFeatureName(name); err != nil || normalized != name {
			return fmt.Errorf("feature name %q is not a kebab-case feature name", name)
		}
		if feature.State != StateBacklog && feature.State != StateConsolidated {
			return fmt.Errorf("feature %s has unknown state %q", name, feature.State)
		}
		if !spec.ValidFingerprint(feature.RequirementsFingerprint) {
			return fmt.Errorf("feature %s has a malformed requirements fingerprint", name)
		}
		identifiers := append(append([]string{}, feature.ActiveIDs...), feature.ReservedIDs...)
		for id := range feature.Statements {
			identifiers = append(identifiers, id)
		}
		for _, id := range identifiers {
			if !recordIdentifier.MatchString(id) {
				return fmt.Errorf("feature %s lists malformed identifier %q", name, id)
			}
		}
	}
	return nil
}
