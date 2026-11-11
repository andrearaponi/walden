package evidence

import (
	"encoding/json"
	"fmt"
)

// The decoder of the ledger as walden v0.14.5 released it: the body of Decode
// in internal/evidence/store.go at the tag v0.14.5 (commit 88aee31ee744),
// statement for statement. It is the reference for what a release before
// proof kinds does with a ledger, so it must stay as that release wrote it:
// do not edit it to follow the decoder of this tree.
//
// Two names are reduced, neither of which the verdict reads: the constant
// SchemaVersion of that release is spelled out, and the records stay raw
// JSON, as fields that release does not know are ignored when it decodes.

const frozenV0145SchemaVersion = "v1alpha2"

type frozenV0145Document struct {
	SchemaVersion string                     `json:"schema_version"`
	Feature       string                     `json:"feature"`
	Tasks         map[string]json.RawMessage `json:"tasks"`
}

func frozenV0145Decode(data []byte, feature string) (frozenV0145Document, error) {
	var document frozenV0145Document
	if err := json.Unmarshal(data, &document); err != nil {
		return frozenV0145Document{}, fmt.Errorf("parse: %w; retain the ledger for diagnosis", err)
	}
	switch document.SchemaVersion {
	case "", "v1alpha1", frozenV0145SchemaVersion:
	default:
		return frozenV0145Document{}, fmt.Errorf("schema %s is unsupported; this binary writes %s — retain the ledger and use a compatible walden reader", document.SchemaVersion, frozenV0145SchemaVersion)
	}
	if document.Feature != "" && document.Feature != feature {
		return frozenV0145Document{}, fmt.Errorf("ledger belongs to feature %q, not %q — retain it and inspect its provenance", document.Feature, feature)
	}
	if document.Tasks == nil {
		document.Tasks = map[string]json.RawMessage{}
	}
	return document, nil
}
