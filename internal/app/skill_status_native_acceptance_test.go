package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/skill"
)

var skillStatusReceiptInputs = []string{
	"internal/skilldist/status.go", "internal/app/skill.go", "internal/output/result.go", "go.mod",
	"skill/walden/SKILL.md", "internal/app/testdata/skill-status-native.json",
	"internal/app/skill_status_native_acceptance_test.go", "docs/reference/cli.md", "docs/reference/json.md", "CHANGELOG.md", "RELEASE_NOTES.md",
}

type skillStatusExpected struct {
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
}

type skillStatusNativeFixture struct {
	ID      string                         `json:"id"`
	Group   string                         `json:"group"`
	Files   map[string]string              `json:"files"`
	Slots   map[string]skillStatusExpected `json:"slots"`
	Warning string                         `json:"warning"`
}

type skillStatusNativeCase struct {
	ID         string            `json:"id"`
	Root       string            `json:"root"`
	Completed  bool              `json:"completed"`
	Skipped    bool              `json:"skipped"`
	JSONExit   *int              `json:"json_exit"`
	TextExit   *int              `json:"text_exit"`
	JSONOutput string            `json:"json_output"`
	TextOutput string            `json:"text_output"`
	JSONStderr string            `json:"json_stderr"`
	TextStderr string            `json:"text_stderr"`
	ReadError  string            `json:"read_error"`
	Before     map[string]string `json:"before"`
	After      map[string]string `json:"after"`
}

type skillStatusNativeReport struct {
	Kind         string                  `json:"kind"`
	Platform     string                  `json:"platform"`
	Arch         string                  `json:"arch"`
	GoVersion    string                  `json:"go_version"`
	Binary       string                  `json:"binary"`
	BinarySHA256 string                  `json:"binary_sha256"`
	GuideSHA256  string                  `json:"guide_sha256"`
	Source       map[string]string       `json:"source"`
	Cases        []skillStatusNativeCase `json:"cases"`
}

func skillStatusDecode(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON document")
	}
	return nil
}

func skillStatusHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func skillStatusFileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return skillStatusHash(data)
}

func skillStatusCurrentInputs(t *testing.T) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, name := range skillStatusReceiptInputs {
		result[name] = skillStatusFileHash(t, filepath.Join(authoringSourceRoot(t), name))
	}
	return result
}

func skillStatusNativeFixtures(t *testing.T) []skillStatusNativeFixture {
	t.Helper()
	var fixtures []skillStatusNativeFixture
	if err := skillStatusDecode(filepath.Join(authoringSourceRoot(t), "internal/app/testdata/skill-status-native.json"), &fixtures); err != nil {
		t.Fatal(err)
	}
	groups, ids := map[string]bool{}, map[string]bool{}
	for _, item := range fixtures {
		if item.ID == "" || ids[item.ID] {
			t.Fatal("duplicate/missing native fixture identity")
		}
		ids[item.ID], groups[item.Group] = true, true
	}
	want := map[string]bool{"file-newlines": true, "marker-endings": true, "shared-block": true, "content-change": true, "absent-target": true, "corrupt-block": true, "read-error": true, "scope-comparison": true}
	if len(fixtures) != 24 || !reflect.DeepEqual(groups, want) {
		t.Fatal("fixed native fixture inventory changed")
	}
	return fixtures
}

// Test-only fixture data, not production parsing. Both the Windows runner and
// this reader bind to the fixed catalog; the reader independently checks seed
// hashes and expected outputs rather than trusting a producer's PASS boolean.
func skillStatusSeed(form string) ([]byte, bool) {
	lf := bytes.ReplaceAll(skill.Content(), []byte("\r\n"), []byte("\n"))
	stamp := []byte("<!-- walden-skill-version: v1.2.3 -->\n")
	joined := func(suffix []byte) []byte { return append(append([]byte(nil), lf...), suffix...) }
	crlf := func(data []byte) []byte { return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n")) }
	switch form {
	case "lf":
		return lf, false
	case "crlf":
		return crlf(lf), false
	case "mixed":
		var result []byte
		for i, line := range bytes.SplitAfter(lf, []byte("\n")) {
			if i%2 == 0 {
				line = crlf(line)
			}
			result = append(result, line...)
		}
		return result, false
	case "stamped-lf":
		return joined(stamp), false
	case "stamped-crlf":
		return crlf(joined(stamp)), false
	case "marker-crlf":
		return joined(crlf(stamp)), false
	case "marker-eof":
		return joined(bytes.TrimSuffix(stamp, []byte("\n"))), false
	case "marker-malformed":
		return joined([]byte("<!-- walden-skill-version: v1.2.3 --!>\n")), false
	case "marker-nontrailing":
		return joined(append(stamp, []byte("AFTER-MARKER-SENTINEL-604\n")...)), false
	case "shared-crlf":
		body := append([]byte("UNRELATED-PREFIX-891\r\n# --- BEGIN WALDEN SKILL ---\r\n"), crlf(joined(stamp))...)
		return append(body, []byte("# --- END WALDEN SKILL ---\r\nUNRELATED-SUFFIX-734\n")...), false
	case "changed-word":
		return crlf(bytes.Replace(lf, []byte("Walden"), []byte("CHANGED-NAME-195"), 1)), false
	case "added":
		return joined([]byte("ADDED-INSTRUCTION-246\n")), false
	case "space":
		return crlf(append([]byte(" "), lf...)), false
	case "bom":
		return append([]byte{0xef, 0xbb, 0xbf}, lf...), false
	case "lone-cr":
		return bytes.Replace(lf, []byte("\n"), []byte("\r"), 1), false
	case "unrelated":
		return []byte("UNRELATED-SHARED-FILE-792\r\n"), false
	case "corrupt":
		return []byte("# --- BEGIN WALDEN SKILL ---\r\nMISSING-END-SENTINEL\r\n"), false
	case "empty":
		return []byte{}, false
	case "read-error":
		return nil, true
	default:
		panic("unknown native fixture form: " + form)
	}
}

var skillStatusNativePaths = map[string]string{
	"claude/user": "home/.claude/skills/walden/SKILL.md", "claude/project": "work/.claude/skills/walden/SKILL.md",
	"codex/user": "home/.codex/AGENTS.md", "codex/project": "work/AGENTS.md",
	"copilot/user": "home/.copilot/skills/walden/SKILL.md", "opencode/user": "home/.config/opencode/skills/walden/SKILL.md",
}

var skillStatusNativeSentinels = map[string]string{
	"home/.agents/.skill-lock.json": "{\"fixture\":true}\r\n", "home/.profile": "PROFILE-SENTINEL-487\r\n", "home/.gitconfig": "[core]\r\n  autocrlf = true\r\n",
}

func checkSkillStatusNativeReport(report skillStatusNativeReport, catalog []skillStatusNativeFixture, source map[string]string, dir string, observed bool) error {
	if observed && report.Kind != "native-windows" {
		return fmt.Errorf("native Windows execution is required")
	}
	if report.Platform != "windows" || (report.Arch != "amd64" && report.Arch != "arm64") || report.GoVersion != runtime.Version() {
		return fmt.Errorf("wrong native platform, architecture or build toolchain")
	}
	if !reflect.DeepEqual(report.Source, source) || report.GuideSHA256 != skillStatusHash(skill.Content()) {
		return fmt.Errorf("native receipt is bound to stale source or guide")
	}
	if !filepath.IsLocal(report.Binary) {
		return fmt.Errorf("binary path must stay within the receipt directory")
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	binary, err := filepath.EvalSymlinks(filepath.Join(dir, report.Binary))
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(base, binary)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("binary symlink escapes the receipt directory")
	}
	data, err := os.ReadFile(binary)
	if err != nil || skillStatusHash(data) != report.BinarySHA256 {
		return fmt.Errorf("missing or stale observed binary")
	}
	fixtures := map[string]skillStatusNativeFixture{}
	for _, fixture := range catalog {
		fixtures[fixture.ID] = fixture
	}
	seen := map[string]bool{}
	for _, result := range report.Cases {
		fixture, ok := fixtures[result.ID]
		if !ok || seen[result.ID] {
			return fmt.Errorf("unknown/duplicate native case %s", result.ID)
		}
		seen[result.ID] = true
		if !result.Completed || result.Skipped || result.JSONExit == nil || result.TextExit == nil || *result.JSONExit != 0 || *result.TextExit != 0 || result.JSONStderr != "" || result.TextStderr != "" {
			return fmt.Errorf("incomplete/failed native command: %s", result.ID)
		}
		root := strings.TrimRight(strings.ReplaceAll(result.Root, "\\", "/"), "/")
		if len(root) < 3 || root[1:3] != ":/" || strings.Contains(root, "/../") {
			return fmt.Errorf("missing Windows fixture root")
		}
		if len(result.Before) == 0 || !reflect.DeepEqual(result.Before, result.After) {
			return fmt.Errorf("missing snapshots or changed fixture: %s", result.ID)
		}
		for path, content := range skillStatusNativeSentinels {
			if !strings.HasPrefix(result.Before[path], "file:"+skillStatusHash([]byte(content))+":") {
				return fmt.Errorf("missing/stale native sentinel: %s/%s", result.ID, path)
			}
		}
		for path, form := range fixture.Files {
			content, directory := skillStatusSeed(form)
			prefix := "file:" + skillStatusHash(content) + ":"
			if directory {
				prefix = "directory:"
				if result.ReadError == "" {
					return fmt.Errorf("read-error antecedent was not exercised: %s", result.ID)
				}
			}
			if !strings.HasPrefix(result.Before[path], prefix) {
				return fmt.Errorf("native input was not the frozen fixture: %s/%s", result.ID, path)
			}
		}
		var envelope struct {
			Schema  string `json:"schema_version"`
			Command string `json:"command"`
			OK      bool   `json:"ok"`
			Result  struct {
				Summary  string           `json:"summary"`
				Exit     *int             `json:"exit_code"`
				Skills   []map[string]any `json:"skills"`
				Warnings []string         `json:"warnings"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(result.JSONOutput), &envelope); err != nil {
			return fmt.Errorf("invalid native output: %s: %w", result.ID, err)
		}
		if envelope.Schema != "v0beta1" || envelope.Command != "skill-status" || !envelope.OK || envelope.Result.Exit == nil || *envelope.Result.Exit != 0 || len(envelope.Result.Skills) != 6 || envelope.Result.Summary != "skill status against embedded version v0.10.5" {
			return fmt.Errorf("invalid native status contract: %s", result.ID)
		}
		slotIDs := map[string]bool{}
		text := strings.ReplaceAll(result.TextOutput, "\\", "/")
		for _, slot := range envelope.Result.Skills {
			agent, a := slot["agent"].(string)
			scope, b := slot["scope"].(string)
			path, p := slot["path"].(string)
			state, s := slot["state"].(string)
			installed, i := slot["installed"].(bool)
			key := agent + "/" + scope
			nativePath, exists := skillStatusNativePaths[key]
			if !a || !b || !p || !s || !i || !exists || slotIDs[key] || !strings.EqualFold(strings.ReplaceAll(path, "\\", "/"), root+"/"+nativePath) {
				return fmt.Errorf("invalid/escaping native slot: %s/%s", result.ID, key)
			}
			slotIDs[key] = true
			want, ok := fixture.Slots[key]
			if !ok {
				want.State = "not-installed"
			}
			if state != want.State || installed != (want.State != "not-installed") {
				return fmt.Errorf("incorrect native classification: %s/%s", result.ID, key)
			}
			version, hasVersion := slot["version"]
			if (want.Version == "" && hasVersion) || (want.Version != "" && version != want.Version) {
				return fmt.Errorf("incorrect marker version: %s/%s", result.ID, key)
			}
			line := fmt.Sprintf("- %s (%s): %s", agent, scope, state)
			if want.Version != "" {
				line += " version=" + want.Version
			}
			if installed {
				line += " path=" + strings.ReplaceAll(path, "\\", "/")
			}
			if !strings.Contains(text, line) {
				return fmt.Errorf("native text/JSON mismatch: %s/%s", result.ID, key)
			}
		}
		warnings := envelope.Result.Warnings
		if (fixture.Warning == "none" && len(warnings) != 0) || (fixture.Warning != "none" && len(warnings) != 1) {
			return fmt.Errorf("incorrect warnings: %s", result.ID)
		}
		if len(warnings) != 0 {
			warning := strings.ReplaceAll(warnings[0], "\\", "/")
			if !strings.Contains(text, warning) {
				return fmt.Errorf("native text omitted its diagnostic: %s", result.ID)
			}
			switch fixture.Warning {
			case "read":
				for _, needle := range []string{"claude", "user", root + "/home/.claude/skills/walden/SKILL.md", "comparison not performed"} {
					if !strings.Contains(warning, needle) {
						return fmt.Errorf("incomplete native read diagnostic: %s", result.ID)
					}
				}
				if strings.Contains(warning, "installations differ") {
					return fmt.Errorf("unavailable body used for scope comparison")
				}
			case "corrupt":
				if !strings.Contains(warning, "END marker") {
					return fmt.Errorf("corrupt block diagnostic missing")
				}
			case "different":
				if !strings.Contains(warning, "installations differ") {
					return fmt.Errorf("real scope divergence was hidden")
				}
			default:
				return fmt.Errorf("unknown warning expectation")
			}
		}
	}
	if len(seen) != len(fixtures) {
		return fmt.Errorf("missing native Windows cases")
	}
	return nil
}

func syntheticSkillStatusNativeReport(t *testing.T, dir string) skillStatusNativeReport {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "binary"), []byte("synthetic fixture only"), 0o600); err != nil {
		t.Fatal(err)
	}
	zero := 0
	r := skillStatusNativeReport{Kind: "synthetic", Platform: "windows", Arch: "amd64", GoVersion: runtime.Version(), Binary: "binary", BinarySHA256: skillStatusFileHash(t, filepath.Join(dir, "binary")), GuideSHA256: skillStatusHash(skill.Content()), Source: skillStatusCurrentInputs(t)}
	for _, fixture := range skillStatusNativeFixtures(t) {
		c := skillStatusNativeCase{ID: fixture.ID, Root: "C:/synthetic/" + fixture.ID, Completed: true, JSONExit: &zero, TextExit: &zero, Before: map[string]string{}}
		for path, data := range skillStatusNativeSentinels {
			c.Before[path] = "file:" + skillStatusHash([]byte(data)) + ":32:synthetic-time"
		}
		for path, form := range fixture.Files {
			data, directory := skillStatusSeed(form)
			if directory {
				c.Before[path] = "directory:16:synthetic-time"
				c.ReadError = "fixture filesystem read failure"
			} else {
				c.Before[path] = "file:" + skillStatusHash(data) + ":32:synthetic-time"
			}
		}
		c.After = map[string]string{}
		for k, v := range c.Before {
			c.After[k] = v
		}
		slots := []map[string]any{}
		for key, path := range skillStatusNativePaths {
			agent, scope, _ := strings.Cut(key, "/")
			want, ok := fixture.Slots[key]
			if !ok {
				want.State = "not-installed"
			}
			slot := map[string]any{"agent": agent, "scope": scope, "path": c.Root + "/" + path, "installed": want.State != "not-installed", "state": want.State}
			if want.Version != "" {
				slot["version"] = want.Version
			}
			slots = append(slots, slot)
			c.TextOutput += fmt.Sprintf("- %s (%s): %s", agent, scope, want.State)
			if want.Version != "" {
				c.TextOutput += " version=" + want.Version
			}
			if want.State != "not-installed" {
				c.TextOutput += " path=" + c.Root + "/" + path
			}
			c.TextOutput += "\n"
		}
		warnings := []string{}
		switch fixture.Warning {
		case "read":
			warnings = append(warnings, "agent claude (user), path "+c.Root+"/home/.claude/skills/walden/SKILL.md: synthetic error; content comparison not performed")
		case "corrupt":
			warnings = append(warnings, "agent codex (user): corrupt walden skill block: "+c.Root+"/home/.codex/AGENTS.md has a BEGIN marker without a matching END marker")
		case "different":
			warnings = append(warnings, "agent claude: user-scope and project-scope installations differ")
		}
		c.TextOutput += strings.Join(warnings, "\n")
		output, err := json.Marshal(map[string]any{"schema_version": "v0beta1", "command": "skill-status", "ok": true, "result": map[string]any{"summary": "skill status against embedded version v0.10.5", "exit_code": 0, "skills": slots, "warnings": warnings}})
		if err != nil {
			t.Fatal(err)
		}
		c.JSONOutput = string(output)
		r.Cases = append(r.Cases, c)
	}
	return r
}

func TestSkillStatusWindowsReportContract(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*skillStatusNativeReport)
	}{
		{"missing case", func(r *skillStatusNativeReport) { r.Cases = r.Cases[1:] }},
		{"duplicate case", func(r *skillStatusNativeReport) { r.Cases[1] = r.Cases[0] }},
		{"unknown case", func(r *skillStatusNativeReport) { r.Cases[0].ID = "unknown" }},
		{"wrong platform", func(r *skillStatusNativeReport) { r.Platform = "darwin" }},
		{"wrong architecture", func(r *skillStatusNativeReport) { r.Arch = "unknown" }},
		{"wrong toolchain", func(r *skillStatusNativeReport) { r.GoVersion = "other" }},
		{"stale source", func(r *skillStatusNativeReport) { r.Source["internal/skilldist/status.go"] = "old" }},
		{"stale binary", func(r *skillStatusNativeReport) { r.BinarySHA256 = strings.Repeat("0", 64) }},
		{"escaping binary", func(r *skillStatusNativeReport) { r.Binary = "../escape" }},
		{"missing binary", func(r *skillStatusNativeReport) { r.Binary = "absent" }},
		{"stale guide", func(r *skillStatusNativeReport) { r.GuideSHA256 = "old" }},
		{"unexecuted", func(r *skillStatusNativeReport) { r.Cases[0].Completed = false }},
		{"skipped", func(r *skillStatusNativeReport) { r.Cases[0].Skipped = true }},
		{"missing exit", func(r *skillStatusNativeReport) { r.Cases[0].JSONExit = nil }},
		{"failed command", func(r *skillStatusNativeReport) { n := 1; r.Cases[0].TextExit = &n }},
		{"missing output", func(r *skillStatusNativeReport) { r.Cases[0].JSONOutput = "" }},
		{"bad state", func(r *skillStatusNativeReport) {
			r.Cases[0].JSONOutput = strings.ReplaceAll(r.Cases[0].JSONOutput, "in-sync", "drifted")
		}},
		{"missing text", func(r *skillStatusNativeReport) { r.Cases[0].TextOutput = "" }},
		{"changed fixture", func(r *skillStatusNativeReport) { r.Cases[0].After["changed.txt"] = "changed" }},
		{"missing snapshots", func(r *skillStatusNativeReport) { r.Cases[0].Before = nil; r.Cases[0].After = nil }},
		{"different input", func(r *skillStatusNativeReport) {
			for k := range r.Cases[0].Before {
				r.Cases[0].Before[k] = "file:bad"
				r.Cases[0].After[k] = "file:bad"
			}
		}},
		{"missing read antecedent", func(r *skillStatusNativeReport) {
			for i := range r.Cases {
				r.Cases[i].ReadError = ""
			}
		}},
	}
	t.Run("valid synthetic structure is not native evidence", func(t *testing.T) {
		dir := t.TempDir()
		r := syntheticSkillStatusNativeReport(t, dir)
		if err := checkSkillStatusNativeReport(r, skillStatusNativeFixtures(t), skillStatusCurrentInputs(t), dir, false); err != nil {
			t.Fatal(err)
		}
		if checkSkillStatusNativeReport(r, skillStatusNativeFixtures(t), skillStatusCurrentInputs(t), dir, true) == nil {
			t.Fatal("synthetic data accepted as native Windows execution")
		}
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r := syntheticSkillStatusNativeReport(t, dir)
			tc.mutate(&r)
			if checkSkillStatusNativeReport(r, skillStatusNativeFixtures(t), skillStatusCurrentInputs(t), dir, false) == nil {
				t.Fatal("invalid native receipt structure accepted")
			}
		})
	}
}

func TestSkillStatusNativeWindowsAcceptance(t *testing.T) {
	path := os.Getenv("WALDEN_SKILL_STATUS_WINDOWS_REPORT")
	if path == "" {
		t.Skip("explicit native Windows receipt not supplied")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(authoringSourceRoot(t), path)
	}
	var report skillStatusNativeReport
	if err := skillStatusDecode(path, &report); err != nil {
		t.Fatal(err)
	}
	if err := checkSkillStatusNativeReport(report, skillStatusNativeFixtures(t), skillStatusCurrentInputs(t), filepath.Dir(path), true); err != nil {
		t.Fatal(err)
	}
	binary := skillStatusBuildCandidate(t, "windows", report.Arch)
	if skillStatusFileHash(t, binary) != report.BinarySHA256 {
		t.Fatal("native Windows executable is not the current equivalent candidate")
	}
}
