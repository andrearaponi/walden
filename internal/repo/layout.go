package repo

// DefaultManagedPaths reports the baseline project paths managed by the CLI
// bootstrap for its own repository structure.
func DefaultManagedPaths() map[string]struct{} {
	return map[string]struct{}{
		".walden":          {},
		"cmd/walden":       {},
		"internal/app":       {},
		"internal/repo":      {},
		"internal/spec":      {},
		"internal/workflow":  {},
		"internal/validation": {},
		"internal/shell":     {},
		"internal/output":    {},
		"internal/testutil":  {},
		"templates/repo":     {},
		"templates/spec":     {},
	}
}
