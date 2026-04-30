package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLintTemplate_MissingManifest(t *testing.T) {
	root := t.TempDir()
	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode() != 2 {
		t.Errorf("ExitCode=%d want 2", r.ExitCode())
	}
	if len(r.Messages) == 0 || r.Messages[0].Level != LintLevelError {
		t.Errorf("expected error message, got %+v", r.Messages)
	}
}

func TestLintTemplate_DuplicateGlob(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = ["scripts/**", "scripts/**"]
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	if err := os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode() != 1 {
		t.Errorf("ExitCode=%d want 1 (warning)", r.ExitCode())
	}
	found := false
	for _, m := range r.Messages {
		if m.Level == LintLevelWarning && strings.Contains(m.Msg, "duplicate glob in overwrite") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-glob warning, got %+v", r.Messages)
	}
}

func TestLintTemplate_MigrationFilenamePattern(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	if err := os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "bad-name.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode() != 2 {
		t.Errorf("ExitCode=%d want 2 (error)", r.ExitCode())
	}
	found := false
	for _, m := range r.Messages {
		if strings.Contains(m.Msg, "filename does not match pattern") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected filename pattern error: %+v", r.Messages)
	}
}

func TestLintTemplate_MigrationMissingHeaders(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	_ = os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644)
	migDir := filepath.Join(root, "migrations")
	_ = os.MkdirAll(migDir, 0o755)
	_ = os.WriteFile(filepath.Join(migDir, "0001-foo.sh"), []byte("#!/usr/bin/env bash\n# migration: 0001-foo\n:\n"), 0o644)

	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range r.Messages {
		if m.Level == LintLevelError && strings.Contains(m.Msg, "missing header keys") {
			found = true
			// Python format: ['idempotent', 'requires', 'touches']
			if !strings.Contains(m.Msg, "'idempotent'") || !strings.Contains(m.Msg, "'requires'") || !strings.Contains(m.Msg, "'touches'") {
				t.Errorf("unexpected list format: %s", m.Msg)
			}
		}
	}
	if !found {
		t.Errorf("expected missing-header-keys error: %+v", r.Messages)
	}
}

func TestLintTemplate_PromptInvalidRisk(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	_ = os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644)
	migDir := filepath.Join(root, "migrations")
	_ = os.MkdirAll(migDir, 0o755)
	body := `---
id: 0001-foo
requires: 0.0.0
scope_glob: content/**
risk: extreme
---
body
`
	_ = os.WriteFile(filepath.Join(migDir, "0001-foo.prompt.md"), []byte(body), 0o644)
	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range r.Messages {
		if m.Level == LintLevelError && strings.Contains(m.Msg, "risk must be") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected risk-error: %+v", r.Messages)
	}
}

func TestLintTemplate_PendingPromptStaleness(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	_ = os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644)
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	_ = os.MkdirAll(pp, 0o755)
	old := filepath.Join(pp, "0001-old.prompt.md")
	_ = os.WriteFile(old, []byte("---\nid: 0001-old\n---\n"), 0o644)
	now := time.Now()
	stale := now.Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(old, stale, stale)
	r, err := LintTemplate(root, now, "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range r.Messages {
		if m.Level == LintLevelWarning && strings.Contains(m.Msg, "pending prompt older than 14 days") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected staleness warning: %+v", r.Messages)
	}
}

func TestLintTemplate_PromptBodyBlockedScope(t *testing.T) {
	root := t.TempDir()
	manifest := `schema_version = 1
template_version = "0.0.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	_ = os.WriteFile(filepath.Join(root, "template.manifest.toml"), []byte(manifest), 0o644)
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	_ = os.MkdirAll(pp, 0o755)
	body := `---
id: 0001-foo
scope_glob: content/**
risk: low
---
text

## Resolved scope
- content/foo.md
- secrets/db.env
`
	_ = os.WriteFile(filepath.Join(pp, "0001-foo.prompt.md"), []byte(body), 0o644)
	r, err := LintTemplate(root, time.Now(), "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range r.Messages {
		if m.Level == LintLevelError && strings.Contains(m.Msg, "resolved scope includes blocked path: secrets/db.env") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected blocked-scope error: %+v", r.Messages)
	}
}
