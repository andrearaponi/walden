package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseDocumentation(t *testing.T) {
	root := authoringSourceRoot(t)
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	notes := read("RELEASE_NOTES.md")
	match := regexp.MustCompile(`^## Walden (v[0-9]+\.[0-9]+\.[0-9]+)\n`).FindStringSubmatch(notes)
	if match == nil {
		t.Fatal("release notes must begin with the release being published")
	}
	version := match[1]
	if !strings.Contains(read("CHANGELOG.md"), "## ["+strings.TrimPrefix(version, "v")+"] - ") {
		t.Errorf("changelog has no dated entry for %s", version)
	}
	if !strings.Contains(read("docs/roadmap.md"), "## Current Release: "+version+"\n") {
		t.Errorf("roadmap does not identify %s", version)
	}
	readme := read("README.md")
	if !strings.Contains(readme, "cmd/walden@"+version) || !strings.Contains(readme, "walden-"+version+"-windows-amd64.exe") {
		t.Errorf("Windows installation example is not aligned with %s", version)
	}
	site := read("site/index.html")
	if !strings.Contains(site, `aria-label="Latest release">`+version+"</a>") {
		t.Errorf("site fallback version is not %s", version)
	}
	for _, text := range []string{"npx skills add andrearaponi/walden --skill walden", "npx skills update walden", "adoption workloads"} {
		if !strings.Contains(site, text) {
			t.Errorf("site omits %q", text)
		}
	}
	for _, obsolete := range []string{"walden skill install", "the skill ships inside the", "then it installs its own AI skill"} {
		if strings.Contains(site, obsolete) {
			t.Errorf("site still advertises removed distribution behavior %q", obsolete)
		}
	}
}
