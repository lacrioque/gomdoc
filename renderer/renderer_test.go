package renderer

import (
	"strings"
	"testing"
)

func TestTransformAdmonitions_Note(t *testing.T) {
	input := `<blockquote>
<p>[!NOTE]<br>
This is a note.</p>
</blockquote>`

	result := string(TransformAdmonitions([]byte(input)))

	if !strings.Contains(result, `class="admonition admonition-note"`) {
		t.Errorf("expected admonition-note class, got:\n%s", result)
	}
	if !strings.Contains(result, `class="admonition-title"`) {
		t.Error("expected admonition-title class")
	}
	if !strings.Contains(result, ">Note<") {
		t.Error("expected title text 'Note'")
	}
	if !strings.Contains(result, "This is a note.") {
		t.Error("expected note body text preserved")
	}
}

func TestTransformAdmonitions_AllTypes(t *testing.T) {
	types := map[string]string{
		"NOTE":      "Note",
		"TIP":       "Tip",
		"IMPORTANT": "Important",
		"WARNING":   "Warning",
		"CAUTION":   "Caution",
		"DANGER":    "Danger",
	}

	for marker, title := range types {
		input := `<blockquote>
<p>[!` + marker + `]<br>
Body text.</p>
</blockquote>`

		result := string(TransformAdmonitions([]byte(input)))
		class := strings.ToLower(marker)

		if !strings.Contains(result, `admonition-`+class) {
			t.Errorf("%s: expected admonition-%s class, got:\n%s", marker, class, result)
		}
		if !strings.Contains(result, ">"+title+"<") {
			t.Errorf("%s: expected title '%s'", marker, title)
		}
	}
}

func TestTransformAdmonitions_MarkerOnlyParagraph(t *testing.T) {
	input := `<blockquote>
<p>[!WARNING]</p>
<p>This is a warning.</p>
</blockquote>`

	result := string(TransformAdmonitions([]byte(input)))

	if !strings.Contains(result, `admonition-warning`) {
		t.Errorf("expected admonition-warning class, got:\n%s", result)
	}
	if !strings.Contains(result, ">Warning<") {
		t.Error("expected title text 'Warning'")
	}
	if !strings.Contains(result, "This is a warning.") {
		t.Error("expected body text preserved")
	}
}

func TestTransformAdmonitions_RegularBlockquote(t *testing.T) {
	input := `<blockquote>
<p>Just a regular quote.</p>
</blockquote>`

	result := string(TransformAdmonitions([]byte(input)))

	if result != input {
		t.Errorf("regular blockquote should be unchanged, got:\n%s", result)
	}
}

func TestTransformAdmonitions_FullRender(t *testing.T) {
	r := New()
	md := []byte(`> [!TIP]
> Use this feature for better results.
`)

	html, err := r.RenderWithLinks(md, "")
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	result := string(html)
	if !strings.Contains(result, `admonition-tip`) {
		t.Errorf("expected admonition-tip in rendered output, got:\n%s", result)
	}
	if !strings.Contains(result, ">Tip<") {
		t.Errorf("expected Tip title in rendered output, got:\n%s", result)
	}
}

func TestRewriteLinks(t *testing.T) {
	input := []byte(`<a href="other.md">Link</a>`)
	result := string(RewriteLinks(input, ""))

	if !strings.Contains(result, `href="/other"`) {
		t.Errorf("expected rewritten link, got: %s", result)
	}
}

func TestParseFrontmatter(t *testing.T) {
	input := []byte("---\ntitle: Hello\nauthor: Test\n---\n# Content")

	fm, rest := ParseFrontmatter(input)

	if fm.Title != "Hello" {
		t.Errorf("expected title 'Hello', got '%s'", fm.Title)
	}
	if fm.Author != "Test" {
		t.Errorf("expected author 'Test', got '%s'", fm.Author)
	}
	if !strings.Contains(string(rest), "# Content") {
		t.Errorf("expected content after frontmatter, got: %s", rest)
	}
}
