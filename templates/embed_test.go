package templates

import (
	"io/fs"
	"strings"
	"testing"
)

func TestRepoFSExposesBootstrapTemplates(t *testing.T) {
	repoFS := RepoFS()

	// Paths are relative to the "repo" subtree, so the "repo/" prefix is gone.
	want := []string{
		"walden/constitution.md",
		"walden/gitignore",
		"walden/lessons.md",
		"github/pull_request_template.md",
		"github/workflows/validate-walden.yml",
	}

	for _, name := range want {
		data, err := fs.ReadFile(repoFS, name)
		if err != nil {
			t.Errorf("ReadFile(%q) failed: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("ReadFile(%q) returned empty content", name)
		}
	}
}

func TestSpecFSExposesDocumentTemplates(t *testing.T) {
	specFS := SpecFS()

	want := []string{
		"requirements.md.tmpl",
		"design.md.tmpl",
		"tasks.md.tmpl",
	}

	for _, name := range want {
		data, err := fs.ReadFile(specFS, name)
		if err != nil {
			t.Errorf("ReadFile(%q) failed: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("ReadFile(%q) returned empty content", name)
		}
	}
}

func TestRepoWorkflowTemplateCarriesVersionToken(t *testing.T) {
	data, err := fs.ReadFile(RepoFS(), "github/workflows/validate-walden.yml")
	if err != nil {
		t.Fatalf("read workflow template: %v", err)
	}
	if !strings.Contains(string(data), "{{WALDEN_VERSION}}") {
		t.Error("workflow template lost the {{WALDEN_VERSION}} pin token (repo init would generate an unpinned install)")
	}
}

func TestSubFilesystemsAreScoped(t *testing.T) {
	// The repo subtree must not leak spec templates and vice versa.
	if _, err := fs.Stat(RepoFS(), "requirements.md.tmpl"); err == nil {
		t.Error("RepoFS unexpectedly exposes spec template requirements.md.tmpl")
	}
	if _, err := fs.Stat(SpecFS(), "walden/constitution.md"); err == nil {
		t.Error("SpecFS unexpectedly exposes repo template walden/constitution.md")
	}
}
