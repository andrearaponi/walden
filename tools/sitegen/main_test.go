package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestRenderSubset locks the Markdown constructs the docs are allowed to use.
func TestRenderSubset(t *testing.T) {
	src := strings.Join([]string{
		"# Title",
		"",
		"A paragraph with **bold**, *italic*, `code`, and a [link](lifecycle.md#the-seal).",
		"",
		"| Col | Meaning |",
		"| --- | --- |",
		"| `a` \\| `b` | escaped pipe |",
		"",
		"- outer",
		"  - inner",
		"- second",
		"",
		"1. first",
		"2. second",
		"",
		"> a quote",
		"",
		"```bash",
		"walden verify <feature>",
		"```",
	}, "\n")

	html, err := render(src, "quickstart.md")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		`<h1 id="title">Title</h1>`,
		"<strong>bold</strong>", "<em>italic</em>", "<code>code</code>",
		`<a href="lifecycle.html#the-seal">link</a>`,
		"<th>Col</th>",
		"<td><code>a</code> | <code>b</code></td>",
		"<ul><li>outer<ul><li>inner</li></ul>\n</li><li>second</li></ul>",
		"<ol><li>first</li><li>second</li></ol>",
		"<blockquote><p>a quote</p></blockquote>",
		`<pre><code class="lang-bash">walden verify &lt;feature&gt;</code></pre>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output lacks %q\n---\n%s", want, html)
		}
	}
}

func TestRenderRefusesUnknownLink(t *testing.T) {
	if _, err := render("[x](no-such-page.md)", "quickstart.md"); err == nil {
		t.Fatal("a link to an unknown page must fail the build")
	}
}

func TestRenderRefusesUnterminatedFence(t *testing.T) {
	if _, err := render("```bash\nnever closed", "quickstart.md"); err == nil {
		t.Fatal("an unterminated fence must fail the build")
	}
}

// TestGenerateRealDocs builds the actual documentation tree, so a doc using
// an unsupported construct or a broken cross-page link fails at PR time —
// not at deploy time.
func TestGenerateRealDocs(t *testing.T) {
	root := repoRoot(t)
	out := t.TempDir()
	if err := generate(filepath.Join(root, "docs"), out); err != nil {
		t.Fatalf("generate over real docs: %v", err)
	}

	// Every generated internal link and anchor must resolve.
	idPattern := regexp.MustCompile(`id="([^"]+)"`)
	hrefPattern := regexp.MustCompile(`href="([^"]+)"`)
	pages := map[string]string{}
	for _, p := range nav {
		raw, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p.out)))
		if err != nil {
			t.Fatalf("missing generated page %s: %v", p.out, err)
		}
		pages[p.out] = string(raw)
	}
	ids := func(page string) map[string]bool {
		set := map[string]bool{}
		for _, m := range idPattern.FindAllStringSubmatch(pages[page], -1) {
			set[m[1]] = true
		}
		return set
	}
	for name, html := range pages {
		for _, m := range hrefPattern.FindAllStringSubmatch(html, -1) {
			href := m[1]
			if strings.HasPrefix(href, "http") {
				continue
			}
			target, anchor, _ := strings.Cut(href, "#")
			if target == "" {
				if !ids(name)[anchor] {
					t.Errorf("%s: dangling local anchor #%s", name, anchor)
				}
				continue
			}
			resolved := filepath.ToSlash(filepath.Join(filepath.Dir(name), target))
			if strings.HasPrefix(resolved, "..") {
				continue // landing page, css, icons: outside the generated tree
			}
			body, ok := pages[resolved]
			if !ok {
				t.Errorf("%s: link to unknown generated page %s", name, href)
				continue
			}
			if anchor != "" && !idPattern.MatchString(body) {
				t.Errorf("%s: target %s has no anchors at all", name, target)
			}
			if anchor != "" && !ids(resolved)[anchor] {
				t.Errorf("%s: anchor #%s missing in %s", name, anchor, target)
			}
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
