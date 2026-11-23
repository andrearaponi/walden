package app

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/validation"
)

// validateFeature is what the validate command judges for one feature: its
// documents, then whether its evidence ledger can be read. review approve
// validates documents only and does not come through here.
func validateFeature(root, name string, scope validation.Scope) (validation.Result, error) {
	result, err := validation.ValidateFeatureWithScope(root, name, scope)
	if err != nil || !result.Valid {
		// A document finding comes first: the ledger is not read.
		return result, err
	}

	if finding := ledgerFinding(root, result.Feature); finding != "" {
		result.Valid = false
		result.Message = "INVALID: " + finding
	}
	return result, nil
}

// conflictMarker is what git writes at the start of the first line of a
// conflict it could not resolve.
var conflictMarker = []byte("<<<<<<< ")

// ledgerFinding says why the evidence ledger of a feature cannot be read. It
// returns "" when the ledger is missing or readable. It reads the file once
// and writes nothing.
func ledgerFinding(root, feature string) string {
	// The path from the repository root, with forward slashes on every
	// platform, so the message reads the same everywhere.
	name := ".walden/evidence/" + feature + ".json"

	data, err := os.ReadFile(evidence.DocumentPath(root, feature))
	if errors.Is(err, fs.ErrNotExist) {
		// A missing ledger is an empty one, as for every other command.
		return ""
	}
	if err != nil {
		cause := err.Error()
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			// The operating system's reason, without the absolute path.
			cause = pathErr.Err.Error()
		}
		return fmt.Sprintf("evidence ledger %s cannot be read: %s", name, cause)
	}

	// The decoder is the one verify, evidence status and release check use,
	// so validate agrees with them about what is readable.
	if _, err := evidence.Decode(data, feature); err != nil {
		if hasConflictMarker(data) {
			// The parser only reports an unexpected character.
			return fmt.Sprintf("evidence ledger %s cannot be read: it holds git conflict markers (%v)", name, err)
		}
		return fmt.Sprintf("evidence ledger %s cannot be read: %v", name, err)
	}
	return ""
}

// hasConflictMarker reports whether a line of the ledger begins with the
// marker git writes at the start of a conflict.
func hasConflictMarker(data []byte) bool {
	return bytes.HasPrefix(data, conflictMarker) ||
		bytes.Contains(data, append([]byte("\n"), conflictMarker...))
}
