package renderer

import (
	"testing"
)

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
