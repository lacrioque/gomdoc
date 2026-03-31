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

func TestParseFrontmatterExtendedFields(t *testing.T) {
	content := []byte(`---
title: Architecture Guide
author: Jane Doe
status: published
date: 2026-03-15
tags: architecture, design, patterns
category: engineering
version: 2.1
reviewers: alice, bob
---
# Architecture Guide

Some content here.
`)

	fm, body := ParseFrontmatter(content)

	if fm.Title != "Architecture Guide" {
		t.Errorf("expected title 'Architecture Guide', got '%s'", fm.Title)
	}
	if fm.Author != "Jane Doe" {
		t.Errorf("expected author 'Jane Doe', got '%s'", fm.Author)
	}
	if fm.Status != "published" {
		t.Errorf("expected status 'published', got '%s'", fm.Status)
	}
	if fm.Date != "2026-03-15" {
		t.Errorf("expected date '2026-03-15', got '%s'", fm.Date)
	}
	if len(fm.Tags) != 3 || fm.Tags[0] != "architecture" || fm.Tags[1] != "design" || fm.Tags[2] != "patterns" {
		t.Errorf("expected tags [architecture, design, patterns], got %v", fm.Tags)
	}
	if fm.Category != "engineering" {
		t.Errorf("expected category 'engineering', got '%s'", fm.Category)
	}
	if fm.Version != "2.1" {
		t.Errorf("expected version '2.1', got '%s'", fm.Version)
	}
	if len(fm.Reviewers) != 2 || fm.Reviewers[0] != "alice" || fm.Reviewers[1] != "bob" {
		t.Errorf("expected reviewers [alice, bob], got %v", fm.Reviewers)
	}

	bodyStr := string(body)
	if bodyStr == "" || bodyStr[0] != '#' {
		t.Errorf("expected body to start with heading, got: %s", bodyStr[:20])
	}
}

func TestParseFrontmatterTagsBracketFormat(t *testing.T) {
	content := []byte(`---
title: Test
tags: [api, rest, http]
---
Body content.
`)

	fm, _ := ParseFrontmatter(content)

	if len(fm.Tags) != 3 || fm.Tags[0] != "api" || fm.Tags[1] != "rest" || fm.Tags[2] != "http" {
		t.Errorf("expected tags [api, rest, http], got %v", fm.Tags)
	}
}

func TestParseFrontmatterNoExtendedFields(t *testing.T) {
	content := []byte(`---
title: Simple Doc
---
Some content.
`)

	fm, _ := ParseFrontmatter(content)

	if fm.Title != "Simple Doc" {
		t.Errorf("expected title 'Simple Doc', got '%s'", fm.Title)
	}
	if fm.Status != "" {
		t.Errorf("expected empty status, got '%s'", fm.Status)
	}
	if len(fm.Tags) != 0 {
		t.Errorf("expected no tags, got %v", fm.Tags)
	}
}
