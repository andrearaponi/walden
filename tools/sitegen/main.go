// Command sitegen renders docs/*.md into site/docs/*.html at Pages deploy
// time. It is deliberately stdlib-only and parses exactly the Markdown
// subset the documentation uses — the docs and the generator are versioned
// together, so an unsupported construct is a build error to fix here, not
// a silent rendering bug to ship.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// page is one documentation page in navigation order.
type page struct {
	src   string // path under docs/
	out   string // path under site/docs/
	title string // sidebar label
	lane  string // sidebar group; empty pages are rendered but unlisted
}

// nav is the single source of the section's information architecture. It
// mirrors docs/README.md's lanes; a page missing from disk fails the build.
var nav = []page{
	{"README.md", "index.html", "Overview", "Start"},
	{"quickstart.md", "quickstart.html", "Quickstart", "Learn"},
	{"agentic.md", "agentic.html", "The Agentic Flow", "Learn"},
	{"lifecycle.md", "lifecycle.html", "The Spec Lifecycle", "Understand"},
	{"boundaries.md", "boundaries.html", "Product Boundaries", "Understand"},
	{"workflow.md", "workflow.html", "The Daily Workflow", "Operate"},
	{"adoption.md", "adoption.html", "Brownfield Adoption", "Operate"},
	{"ci.md", "ci.html", "CI Integration", "Operate"},
	{"reference/cli.md", "reference/cli.html", "CLI Commands", "Reference"},
	{"reference/json.md", "reference/json.html", "JSON Contract", "Reference"},
	{"reference/spec-format.md", "reference/spec-format.html", "Spec File Format", "Reference"},
	{"roadmap.md", "roadmap.html", "Roadmap", "Project"},
	{"concepts.md", "concepts.html", "Concepts", ""}, // pointer page: kept for old links, unlisted
}

const repoBlobURL = "https://github.com/andrearaponi/walden/blob/main/"

func main() {
	docsDir, outDir := "docs", filepath.Join("site", "docs")
	if len(os.Args) == 3 {
		docsDir, outDir = os.Args[1], os.Args[2]
	}
	if err := generate(docsDir, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "sitegen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("sitegen: %d pages → %s\n", len(nav), outDir)
}

func generate(docsDir, outDir string) error {
	for _, p := range nav {
		raw, err := os.ReadFile(filepath.Join(docsDir, p.src))
		if err != nil {
			return fmt.Errorf("read %s: %w", p.src, err)
		}
		body, err := render(string(raw), p.src)
		if err != nil {
			return fmt.Errorf("render %s: %w", p.src, err)
		}
		html := pageHTML(p, body)
		dest := filepath.Join(outDir, filepath.FromSlash(p.out))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", p.out, err)
		}
		if err := os.WriteFile(dest, []byte(html), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", p.out, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Markdown subset renderer
// ---------------------------------------------------------------------------

var (
	headerPattern = regexp.MustCompile(`^(#{1,4}) (.+)$`)
	fencePattern  = regexp.MustCompile("^```([a-z]*)$")
	olPattern     = regexp.MustCompile(`^(\s*)(\d+)\. (.+)$`)
	ulPattern     = regexp.MustCompile(`^(\s*)- (.+)$`)
	tableRow      = regexp.MustCompile(`^\|.*\|$`)
	tableSep      = regexp.MustCompile(`^\|[\s:|-]+\|$`)
)

// render converts one document body to HTML. srcPath (relative to docs/)
// determines how relative links are rewritten. Link resolution panics
// surface as ordinary errors.
func render(src, srcPath string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return renderBody(src, srcPath)
}

func renderBody(src, srcPath string) (string, error) {
	lines := strings.Split(src, "\n")
	var out strings.Builder
	var para []string
	var listStack []string // open list tags, innermost last

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		out.WriteString("<p>" + inline(strings.Join(para, " "), srcPath) + "</p>\n")
		para = nil
	}
	closeLists := func(depth int) {
		for len(listStack) > depth {
			tag := listStack[len(listStack)-1]
			listStack = listStack[:len(listStack)-1]
			out.WriteString("</li></" + tag + ">\n")
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimRight(line, " ")

		// Fenced code blocks take everything verbatim until the closing fence.
		if m := fencePattern.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			closeLists(0)
			var code []string
			for i++; i < len(lines); i++ {
				if strings.TrimRight(lines[i], " ") == "```" {
					break
				}
				code = append(code, lines[i])
			}
			if i == len(lines) {
				return "", fmt.Errorf("unterminated code fence")
			}
			lang := ""
			if m[1] != "" {
				lang = ` class="lang-` + m[1] + `"`
			}
			out.WriteString("<pre><code" + lang + ">" + escape(strings.Join(code, "\n")) + "</code></pre>\n")
			continue
		}

		if trimmed == "" {
			flushPara()
			closeLists(0)
			continue
		}

		if m := headerPattern.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			closeLists(0)
			level := len(m[1])
			out.WriteString(fmt.Sprintf("<h%d id=%q>%s</h%d>\n", level, slug(m[2]), inline(m[2], srcPath), level))
			continue
		}

		if strings.HasPrefix(trimmed, "> ") {
			flushPara()
			closeLists(0)
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(lines[i], "> "); i++ {
				quote = append(quote, strings.TrimPrefix(lines[i], "> "))
			}
			i--
			out.WriteString("<blockquote><p>" + inline(strings.Join(quote, " "), srcPath) + "</p></blockquote>\n")
			continue
		}

		// Tables: a pipe row followed by a separator row.
		if tableRow.MatchString(trimmed) && i+1 < len(lines) && tableSep.MatchString(strings.TrimRight(lines[i+1], " ")) {
			flushPara()
			closeLists(0)
			out.WriteString("<div class=\"table-scroll\"><table>\n<thead><tr>")
			for _, cell := range splitRow(trimmed) {
				out.WriteString("<th>" + inline(cell, srcPath) + "</th>")
			}
			out.WriteString("</tr></thead>\n<tbody>\n")
			for i += 2; i < len(lines) && tableRow.MatchString(strings.TrimRight(lines[i], " ")); i++ {
				out.WriteString("<tr>")
				for _, cell := range splitRow(strings.TrimRight(lines[i], " ")) {
					out.WriteString("<td>" + inline(cell, srcPath) + "</td>")
				}
				out.WriteString("</tr>\n")
			}
			i--
			out.WriteString("</tbody></table></div>\n")
			continue
		}

		// Lists: two indentation levels, ordered and unordered.
		if m := ulPattern.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			handleListItem(&out, &listStack, "ul", len(m[1])/2, m[2], srcPath, closeLists)
			continue
		}
		if m := olPattern.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			handleListItem(&out, &listStack, "ol", len(m[1])/2, m[3], srcPath, closeLists)
			continue
		}

		// A further-indented plain line inside a list continues the item.
		if len(listStack) > 0 && strings.HasPrefix(line, "  ") {
			out.WriteString(" " + inline(strings.TrimSpace(line), srcPath))
			continue
		}

		para = append(para, trimmed)
	}
	flushPara()
	closeLists(0)
	return out.String(), nil
}

func handleListItem(out *strings.Builder, stack *[]string, tag string, depth int, text, srcPath string, closeLists func(int)) {
	want := depth + 1
	closeLists(want)
	for len(*stack) < want {
		out.WriteString("<" + tag + "><li>")
		*stack = append(*stack, tag)
	}
	// Close the previous sibling item unless this <li> was just opened.
	if !strings.HasSuffix(out.String(), "<li>") {
		out.WriteString("</li><li>")
	}
	out.WriteString(inline(text, srcPath))
}

// splitRow splits a table row on unescaped pipes; `\|` renders literally.
func splitRow(row string) []string {
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	row = strings.ReplaceAll(row, `\|`, "\x00")
	cells := strings.Split(row, "|")
	for i := range cells {
		cells[i] = strings.ReplaceAll(strings.TrimSpace(cells[i]), "\x00", "|")
	}
	return cells
}

var (
	codeSpan    = regexp.MustCompile("`([^`]+)`")
	linkSpan    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	boldSpan    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicSpan  = regexp.MustCompile(`\*([^*]+)\*`)
	placeholder = regexp.MustCompile("\x00(\\d+)\x00")
)

// inline renders spans: code first (protected from further processing),
// then links, bold, italic, over HTML-escaped text.
func inline(text, srcPath string) string {
	var protected []string
	text = codeSpan.ReplaceAllStringFunc(text, func(m string) string {
		inner := codeSpan.FindStringSubmatch(m)[1]
		protected = append(protected, "<code>"+escape(inner)+"</code>")
		return fmt.Sprintf("\x00%d\x00", len(protected)-1)
	})
	text = escape(text)
	text = linkSpan.ReplaceAllStringFunc(text, func(m string) string {
		parts := linkSpan.FindStringSubmatch(m)
		return `<a href="` + rewriteLink(parts[2], srcPath) + `">` + parts[1] + `</a>`
	})
	text = boldSpan.ReplaceAllString(text, "<strong>$1</strong>")
	text = italicSpan.ReplaceAllString(text, "<em>$1</em>")
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		var idx int
		fmt.Sscanf(m, "\x00%d\x00", &idx)
		return protected[idx]
	})
}

// rewriteLink maps documentation links onto the generated site: .md targets
// become .html, links escaping docs/ point at the repository on GitHub.
func rewriteLink(href, srcPath string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "#") {
		return href
	}
	target, anchor, _ := strings.Cut(href, "#")
	if anchor != "" {
		anchor = "#" + anchor
	}
	resolved := filepath.ToSlash(filepath.Join(filepath.Dir(srcPath), target))
	if strings.HasPrefix(resolved, "../") {
		return repoBlobURL + strings.TrimPrefix(resolved, "../") + anchor
	}
	for _, p := range nav {
		if p.src == resolved {
			rel, err := filepath.Rel(filepath.Dir(filepath.FromSlash(srcPath)), filepath.FromSlash(p.out))
			if err != nil {
				panic(fmt.Sprintf("relativize %s from %s: %v", p.out, srcPath, err))
			}
			return filepath.ToSlash(rel) + anchor
		}
	}
	panic(fmt.Sprintf("link to unknown page %q in %s", href, srcPath))
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// slug mirrors GitHub's header anchors so cross-page fragment links written
// for the repository view keep working on the site.
var slugStrip = regexp.MustCompile(`[^\w\s-]`)

func slug(header string) string {
	s := strings.ToLower(strings.TrimSpace(header))
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "*", "")
	s = slugStrip.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), "-")
}

// ---------------------------------------------------------------------------
// Page template
// ---------------------------------------------------------------------------

func pageHTML(current page, body string) string {
	depth := strings.Count(current.out, "/")
	root := strings.Repeat("../", depth) // to site/docs/
	site := root + "../"                 // to site/

	var sidebar strings.Builder
	lane := ""
	for _, p := range nav {
		if p.lane == "" {
			continue
		}
		if p.lane != lane {
			if lane != "" {
				sidebar.WriteString("</ul>\n")
			}
			lane = p.lane
			sidebar.WriteString(`<p class="lane">` + escape(lane) + "</p>\n<ul>\n")
		}
		class := ""
		if p.src == current.src {
			class = ` class="here" aria-current="page"`
		}
		sidebar.WriteString(`<li><a` + class + ` href="` + root + p.out + `">` + escape(p.title) + "</a></li>\n")
	}
	sidebar.WriteString("</ul>\n")

	title := current.title
	if current.src == "README.md" {
		title = "Documentation"
	}

	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>` + escape(title) + ` · Walden Docs</title>
  <meta name="description" content="Walden documentation — the spec-driven delivery kernel: lifecycle, workflow, adoption, and full CLI/JSON references." />
  <link rel="icon" href="` + site + `walden.png" />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,380;9..144,500;9..144,600&family=JetBrains+Mono:wght@400;500;700&display=swap" rel="stylesheet" />
  <link rel="stylesheet" href="` + site + `docs.css" />
</head>
<body>
  <header class="site-head">
    <div class="brand">
      <a class="wordmark" href="` + site + `index.html">Walden<span class="mark">§</span></a>
      <a class="version-tag" href="https://github.com/andrearaponi/walden/releases/latest">docs</a>
    </div>
    <nav class="nav" aria-label="Primary">
      <a href="` + root + `index.html">Docs</a>
      <a href="` + site + `index.html#install">Install</a>
      <a href="https://github.com/andrearaponi/walden">GitHub&nbsp;↗</a>
    </nav>
  </header>
  <div class="layout">
    <aside class="sidebar" aria-label="Documentation">
` + sidebar.String() + `    </aside>
    <main class="doc">
` + body + `    </main>
  </div>
  <footer class="foot">
    <span>intention before code · proof before completion</span>
    <a href="https://github.com/andrearaponi/walden/tree/main/docs">Edit these pages on GitHub ↗</a>
  </footer>
</body>
</html>
`
}
