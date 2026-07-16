package spec

import (
	"fmt"
	"sort"
	"strings"
)

// DocumentSchemaVersion is the document schema this CLI reads and writes.
// Every save stamps it, so repositories converge to versioned documents
// through normal use. Documents without the field are accepted as legacy;
// documents declaring a different version are refused with the remedy.
const DocumentSchemaVersion = "v1alpha1"

// checkDocumentSchemaVersion refuses documents this CLI cannot honor. The
// version check runs before any field-level validation: a document written
// by a newer CLI is reported as a version mismatch, not as unknown fields.
func checkDocumentSchemaVersion(path string, fields map[string]string) error {
	declared := fields["walden_schema_version"]
	if declared != "" && declared != DocumentSchemaVersion {
		return fmt.Errorf("%s declares walden_schema_version %q; this CLI supports %q — upgrade the walden CLI or migrate the document", path, declared, DocumentSchemaVersion)
	}
	return nil
}

// checkUnknownFrontmatterFields replaces silent deletion with a loud,
// remediable error: the field the writer would drop is exactly the field the
// loader refuses. Namespaced `x-` extensions and the schema version are
// always legal; the writer's own allowlist defines what "core" means.
func checkUnknownFrontmatterFields(path string, fields map[string]string) error {
	coreKeys, err := orderedFrontmatterKeys(path)
	if err != nil {
		return err
	}
	core := map[string]bool{"walden_schema_version": true}
	for _, key := range coreKeys {
		core[key] = true
	}

	unknown := []string{}
	for key := range fields {
		if core[key] || strings.HasPrefix(key, "x-") {
			continue
		}
		unknown = append(unknown, key)
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s contains unknown frontmatter field %q — rename it to %q to preserve it, or remove it", path, unknown[0], "x-"+unknown[0])
}
