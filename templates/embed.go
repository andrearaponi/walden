package templates

import (
	"embed"
	"io/fs"
)

//go:embed repo spec
var embedded embed.FS

// RepoFS returns the embedded bootstrap templates for repository initialization.
func RepoFS() fs.FS {
	repoFS, err := fs.Sub(embedded, "repo")
	if err != nil {
		panic(err)
	}

	return repoFS
}

// SpecFS returns the embedded document templates for feature scaffolding.
func SpecFS() fs.FS {
	specFS, err := fs.Sub(embedded, "spec")
	if err != nil {
		panic(err)
	}

	return specFS
}
