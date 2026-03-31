package renderer

import (
	"testing"
)

func TestParseFrontmatterBasic(t *testing.T) {
	content := []byte("---\ntitle: My Doc\nauthor: Alice\n---\n# Hello\n")
	fm, body := ParseFrontmatter(content)

	if fm.Title != "My Doc" {
		t.Errorf("expected title 'My Doc', got '%s'", fm.Title)
	}
	if fm.Author != "Alice" {
		t.Errorf("expected author 'Alice', got '%s'", fm.Author)
	}
	if string(body) != "# Hello\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestParseFrontmatterExtended(t *testing.T) {
	content := []byte(`---
title: Architecture Guide
author: Jane Doe
status: approved
date: 2026-01-15
tags: architecture, deployment, kubernetes
category: infrastructure
version: 2.1.0
reviewers: Alice, Bob
---
# Architecture

Content here.
`)
	fm, _ := ParseFrontmatter(content)

	if fm.Title != "Architecture Guide" {
		t.Errorf("expected title 'Architecture Guide', got '%s'", fm.Title)
	}
	if fm.Status != "approved" {
		t.Errorf("expected status 'approved', got '%s'", fm.Status)
	}
	if fm.Date != "2026-01-15" {
		t.Errorf("expected date '2026-01-15', got '%s'", fm.Date)
	}
	if len(fm.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(fm.Tags), fm.Tags)
	}
	if fm.Tags[0] != "architecture" || fm.Tags[1] != "deployment" || fm.Tags[2] != "kubernetes" {
		t.Errorf("unexpected tags: %v", fm.Tags)
	}
	if fm.Category != "infrastructure" {
		t.Errorf("expected category 'infrastructure', got '%s'", fm.Category)
	}
	if fm.Version != "2.1.0" {
		t.Errorf("expected version '2.1.0', got '%s'", fm.Version)
	}
	if len(fm.Reviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %d: %v", len(fm.Reviewers), fm.Reviewers)
	}
	if fm.Reviewers[0] != "Alice" || fm.Reviewers[1] != "Bob" {
		t.Errorf("unexpected reviewers: %v", fm.Reviewers)
	}
}

func TestParseFrontmatterYAMLListSyntax(t *testing.T) {
	content := []byte(`---
title: Test
tags:
  - go
  - testing
  - mcp
reviewers:
  - Alice
  - Bob
---
Body
`)
	fm, _ := ParseFrontmatter(content)

	if len(fm.Tags) != 3 {
		t.Fatalf("expected 3 tags from YAML list, got %d: %v", len(fm.Tags), fm.Tags)
	}
	if fm.Tags[0] != "go" || fm.Tags[1] != "testing" || fm.Tags[2] != "mcp" {
		t.Errorf("unexpected tags: %v", fm.Tags)
	}
	if len(fm.Reviewers) != 2 {
		t.Fatalf("expected 2 reviewers from YAML list, got %d: %v", len(fm.Reviewers), fm.Reviewers)
	}
}

func TestParseFrontmatterInlineListSyntax(t *testing.T) {
	content := []byte("---\ntitle: Test\ntags: [go, testing, mcp]\n---\nBody\n")
	fm, _ := ParseFrontmatter(content)

	if len(fm.Tags) != 3 {
		t.Fatalf("expected 3 tags from inline list, got %d: %v", len(fm.Tags), fm.Tags)
	}
	if fm.Tags[0] != "go" || fm.Tags[1] != "testing" || fm.Tags[2] != "mcp" {
		t.Errorf("unexpected tags: %v", fm.Tags)
	}
}

func TestParseFrontmatterNoFrontmatter(t *testing.T) {
	content := []byte("# Hello\nNo frontmatter here.\n")
	fm, body := ParseFrontmatter(content)

	if fm.Title != "" {
		t.Errorf("expected empty title, got '%s'", fm.Title)
	}
	if string(body) != string(content) {
		t.Errorf("expected body to equal original content")
	}
}

func TestParseFrontmatterQuotedValues(t *testing.T) {
	content := []byte("---\ntitle: \"Quoted Title\"\nstatus: 'draft'\n---\nBody\n")
	fm, _ := ParseFrontmatter(content)

	if fm.Title != "Quoted Title" {
		t.Errorf("expected title 'Quoted Title', got '%s'", fm.Title)
	}
	if fm.Status != "draft" {
		t.Errorf("expected status 'draft', got '%s'", fm.Status)
	}
}
