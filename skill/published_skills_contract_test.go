package skill

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// Published-skill compliance (legacy-skill-copies, R6). A published skill is a
// git-tracked skill/<dir>/SKILL.md: the set comes from git ls-files, never from
// the working tree, so a gitignored local skill can neither satisfy nor escape
// a check. Every checker is a pure function over content, and each subtest
// runs it on a fixture it must reject before it runs on the real files, so a
// broken matcher fails loudly instead of passing vacuously.
func TestPublishedSkillsCompliance(t *testing.T) {
	root := authoringRoot(t)
	tracked := trackedSkillFiles(t, root)
	published := publishedSkillDirs(tracked)
	if len(published) == 0 {
		t.Fatal("git tracks no skill/<dir>/SKILL.md")
	}
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	// filesOf lists the tracked files of one published skill directory.
	filesOf := func(dir string) []string {
		var files []string
		for _, rel := range tracked {
			if strings.HasPrefix(rel, "skill/"+dir+"/") {
				files = append(files, rel)
			}
		}
		return files
	}

	t.Run("frontmatter", func(t *testing.T) {
		rejects := map[string]string{
			"uppercase name":        "---\nname: Walden\ndescription: \"guide\"\n---\n",
			"name differs from dir": "---\nname: walden-other\ndescription: \"guide\"\n---\n",
			"empty description":     "---\nname: walden\ndescription: \"\"\n---\n",
			"1025-char description": "---\nname: walden\ndescription: \"" + strings.Repeat("d", 1025) + "\"\n---\n",
			"unknown top-level key": "---\nname: walden\ndescription: \"guide\"\nversion: 1\n---\n",
			"no frontmatter at all": "# Walden\n",
		}
		for name, fixture := range rejects {
			if err := checkSkillFrontmatter("walden", fixture); err == nil {
				t.Errorf("fixture %q was accepted", name)
			}
		}
		if err := checkSkillFrontmatter("walden", "---\nname: walden\ndescription: \"guide\"\nmetadata:\n  short-description: x\n---\n"); err != nil {
			t.Errorf("a compliant fixture was rejected: %v", err)
		}
		for _, dir := range published {
			if err := checkSkillFrontmatter(dir, read("skill/"+dir+"/SKILL.md")); err != nil {
				t.Errorf("skill/%s/SKILL.md: %v", dir, err)
			}
		}
	})

	t.Run("references-stay-inside", func(t *testing.T) {
		if err := checkReferencesStayInside("See the [README](../../README.md#x) first.\n"); err == nil {
			t.Error("a link to ../../README.md was accepted")
		}
		if err := checkReferencesStayInside("See [install](install.md) and [the site](https://skills.sh).\n"); err != nil {
			t.Errorf("inside references were rejected: %v", err)
		}
		for _, dir := range published {
			for _, rel := range filesOf(dir) {
				if err := checkReferencesStayInside(read(rel)); err != nil {
					t.Errorf("%s: %v", rel, err)
				}
			}
		}
	})

	t.Run("no-fetch-and-execute", func(t *testing.T) {
		// The v0.10.2 guide's installer line, verbatim.
		const installerLine = `  curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/v0.10.2/install.sh -o "$installer_dir/install.sh"`
		for name, fixture := range map[string]string{
			"raw.githubusercontent.com script": installerLine + "\n",
			"curl piped into sh":               "curl -fsSL https://example.com/install.sh | sh\n",
			"wget piped into sudo bash":        "wget -qO- https://example.com/x | sudo bash -s -- --flag\n",
		} {
			if err := checkNoFetchAndExecute(fixture); err == nil {
				t.Errorf("fixture %q was accepted", name)
			}
		}
		if err := checkNoFetchAndExecute("Open https://github.com/andrearaponi/walden; the README has the installer.\n"); err != nil {
			t.Errorf("a pointer without a fetch was rejected: %v", err)
		}
		for _, dir := range published {
			for _, rel := range filesOf(dir) {
				if err := checkNoFetchAndExecute(read(rel)); err != nil {
					t.Errorf("%s: %v", rel, err)
				}
			}
		}
	})

	t.Run("named-skills-are-published", func(t *testing.T) {
		if err := checkNamedSkillsPublished("the companion skills `walden-history` and `walden-soundings`", published); err == nil {
			t.Error("a sentence naming walden-soundings was accepted")
		}
		if err := checkNamedSkillsPublished("`walden-history`, the stamp `walden-skill-version` and the image `walden-og.jpg`", published); err != nil {
			t.Errorf("a published skill, the stamp token and a file name were rejected: %v", err)
		}
		documents := []string{"skill/walden/install.md", "skill/doc.go", "README.md"}
		err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".md") {
				rel, _ := filepath.Rel(root, path)
				documents = append(documents, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range documents {
			if err := checkNamedSkillsPublished(read(rel), published); err != nil {
				t.Errorf("%s: %v", rel, err)
			}
		}
	})

	t.Run("release-notes-corrected", func(t *testing.T) {
		// The v0.11.0 sentences as released.
		const changelogAsReleased = "## [0.11.0] - 2026-09-22\n\n### Changed\n\n- **One distribution channel for the guide.** The `walden` skill is installed and updated through the Skills CLI (`npx skills add andrearaponi/walden`, `npx skills update walden`), exactly like the companion skills `walden-history` and `walden-soundings`.\n\n## [0.10.5] - 2026-09-20\n"
		const notesAsReleased = "## Walden v0.11.0\n\nInstall the guide with `npx skills add andrearaponi/walden` and update it with `npx skills update walden`. This is the same model the companion skills `walden-history` and `walden-soundings` already used.\n\n## Walden v0.10.5\n"
		if err := checkSectionFreeOf(changelogAsReleased, "## [0.11.0]", "## [", "walden-soundings"); err == nil {
			t.Error("the v0.11.0 changelog entry as released was accepted")
		}
		if err := checkSectionFreeOf(notesAsReleased, "## Walden v0.11.0", "## Walden ", "walden-soundings"); err == nil {
			t.Error("the v0.11.0 release notes as released were accepted")
		}
		if err := checkSectionFreeOf(read("CHANGELOG.md"), "## [0.11.0]", "## [", "walden-soundings"); err != nil {
			t.Errorf("CHANGELOG.md: %v", err)
		}
		if err := checkSectionFreeOf(read("RELEASE_NOTES.md"), "## Walden v0.11.0", "## Walden ", "walden-soundings"); err != nil {
			t.Errorf("RELEASE_NOTES.md: %v", err)
		}
	})

	t.Run("correction-recorded", func(t *testing.T) {
		if err := checkCorrectionRecorded("## [Unreleased]\n\n### Added\n\n- Something else.\n\n## [0.13.0] - 2026-10-08\n"); err == nil {
			t.Error("an [Unreleased] section without the correction was accepted")
		}
		if err := checkCorrectionRecorded(read("CHANGELOG.md")); err != nil {
			t.Errorf("CHANGELOG.md: %v", err)
		}
	})
}

// trackedSkillFiles lists the git-tracked files under skill/, slash-separated
// and relative to the repository root.
func trackedSkillFiles(t *testing.T, root string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "ls-files", "skill").Output()
	if err != nil {
		t.Fatalf("git ls-files skill: %v", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, filepath.ToSlash(line))
		}
	}
	return files
}

// publishedSkillDirs returns the directories that hold a tracked SKILL.md
// directly under skill/, sorted.
func publishedSkillDirs(tracked []string) []string {
	var dirs []string
	for _, rel := range tracked {
		parts := strings.Split(rel, "/")
		if len(parts) == 3 && parts[0] == "skill" && parts[2] == "SKILL.md" {
			dirs = append(dirs, parts[1])
		}
	}
	sort.Strings(dirs)
	return dirs
}

var (
	skillNamePattern      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	frontmatterKeyLine    = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*):[ \t]*(.*)$`)
	allowedFrontmatterKey = map[string]bool{"name": true, "description": true, "license": true, "compatibility": true, "metadata": true, "allowed-tools": true}
)

// checkSkillFrontmatter applies the Agent Skills frontmatter rules (R6.AC1) to
// a SKILL.md body whose directory is dir.
func checkSkillFrontmatter(dir, content string) error {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return fmt.Errorf("no frontmatter block")
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fmt.Errorf("unterminated frontmatter block")
	}
	fields := map[string]string{}
	for _, line := range strings.Split(rest[:end], "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue // nested metadata lines and comments are not top-level keys
		}
		match := frontmatterKeyLine.FindStringSubmatch(line)
		if match == nil {
			return fmt.Errorf("frontmatter line %q is not a top-level key", line)
		}
		if !allowedFrontmatterKey[match[1]] {
			return fmt.Errorf("frontmatter key %q is not allowed", match[1])
		}
		fields[match[1]] = match[2]
	}
	name := strings.TrimSpace(fields["name"])
	switch {
	case name == "":
		return fmt.Errorf("frontmatter has no name")
	case !skillNamePattern.MatchString(name) || len(name) > 64:
		return fmt.Errorf("name %q is not 1–64 lowercase letters, digits and single hyphens", name)
	case name != dir:
		return fmt.Errorf("name %q differs from directory %q", name, dir)
	}
	description := unquoteFrontmatter(fields["description"])
	if n := utf8.RuneCountInString(description); n < 1 || n > 1024 {
		return fmt.Errorf("description has %d characters, want 1–1024", n)
	}
	return nil
}

// unquoteFrontmatter strips one pair of surrounding quotes and unescapes the
// quotes inside, which is how a long description is written in the guides.
func unquoteFrontmatter(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
		quote := value[:1]
		value = value[1 : len(value)-1]
		value = strings.ReplaceAll(value, `\`+quote, quote)
	}
	return value
}

var markdownLinkTarget = regexp.MustCompile(`\]\(\s*([^)\s]+)`)

// checkReferencesStayInside rejects a Markdown link whose target leaves the
// skill directory (R6.AC2).
func checkReferencesStayInside(content string) error {
	for _, match := range markdownLinkTarget.FindAllStringSubmatch(content, -1) {
		target := match[1]
		if strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
			return fmt.Errorf("link target %q leaves the skill directory", target)
		}
	}
	return nil
}

var (
	fetchPipedIntoShell = regexp.MustCompile(`\b(?:curl|wget)\b[^\n]*\|\s*(?:sudo\s+)?(?:sh|bash|zsh|dash)\b`)
	rawGitHubScript     = regexp.MustCompile(`raw\.githubusercontent\.com`)
)

// checkNoFetchAndExecute rejects an instruction that fetches and runs remote
// code (R6.AC3).
func checkNoFetchAndExecute(content string) error {
	if match := fetchPipedIntoShell.FindString(content); match != "" {
		return fmt.Errorf("fetch piped into a shell: %q", strings.TrimSpace(match))
	}
	if match := rawGitHubScript.FindString(content); match != "" {
		return fmt.Errorf("names a raw.githubusercontent.com script")
	}
	return nil
}

var namedSkillToken = regexp.MustCompile(`\bwalden-([a-z]+(?:-[a-z]+)*)\b`)

// checkNamedSkillsPublished requires every walden-<name> skill named in the
// content to be a published skill (R6.AC4). A token followed by "." and a
// letter is a file name, and walden-skill-version is the legacy stamp token,
// so neither names a skill.
func checkNamedSkillsPublished(content string, published []string) error {
	isPublished := map[string]bool{}
	for _, dir := range published {
		isPublished[dir] = true
	}
	for _, loc := range namedSkillToken.FindAllStringSubmatchIndex(content, -1) {
		token := content[loc[0]:loc[1]]
		if token == "walden-skill-version" {
			continue
		}
		if loc[1]+1 < len(content) && content[loc[1]] == '.' && isASCIILetter(content[loc[1]+1]) {
			continue
		}
		if !isPublished[token] {
			return fmt.Errorf("names %q, which is not a published skill", token)
		}
	}
	return nil
}

func isASCIILetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// checkSectionFreeOf finds the section that starts with heading and ends at
// the next heading with nextPrefix, and rejects it when it contains needle
// (R6.AC5).
func checkSectionFreeOf(document, heading, nextPrefix, needle string) error {
	start := strings.Index(document, heading)
	if start < 0 {
		return fmt.Errorf("no section %q", heading)
	}
	section := document[start+len(heading):]
	if next := strings.Index(section, "\n"+nextPrefix); next >= 0 {
		section = section[:next]
	}
	if strings.Contains(section, needle) {
		return fmt.Errorf("section %q still names %s", heading, needle)
	}
	return nil
}

// checkCorrectionRecorded requires the [Unreleased] section of the changelog
// to carry a Fixed entry naming walden-soundings as not published (R6.AC6).
func checkCorrectionRecorded(changelog string) error {
	start := strings.Index(changelog, "## [Unreleased]")
	if start < 0 {
		return fmt.Errorf("no [Unreleased] section")
	}
	section := changelog[start:]
	if next := strings.Index(section[1:], "\n## ["); next >= 0 {
		section = section[:next+1]
	}
	fixed := strings.Index(section, "### Fixed")
	if fixed < 0 {
		return fmt.Errorf("[Unreleased] has no Fixed entries")
	}
	if !strings.Contains(section[fixed:], "walden-soundings") {
		return fmt.Errorf("[Unreleased] Fixed entries do not record the walden-soundings correction")
	}
	return nil
}
