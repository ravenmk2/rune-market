package designmd

import (
	"strings"
	"testing"
)

const fullDoc = `# My Design System

## Overview

A design language for demo apps.

## Colors

Primary: #1a73e8, background: #ffffff, surface: #F8F9FA.
Accent: #ff5722 and short #abc forms.

## Typography

Inter for everything.

## Spacing

4px grid.

## Components

Buttons, cards, dialogs.

## Elevation

Three shadow levels.

## Guidelines

Be consistent.
`

func levels(r *Report, level string) []Check {
	var out []Check
	for _, c := range r.Checks {
		if c.Level == level {
			out = append(out, c)
		}
	}
	return out
}

func TestValidateFullDoc(t *testing.T) {
	r := Validate([]byte(fullDoc))
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %+v", levels(r, LevelError))
	}
	if len(levels(r, LevelWarn)) != 0 {
		t.Fatalf("unexpected warnings: %+v", levels(r, LevelWarn))
	}
	// hex color counting inside the Colors section
	var colorCheck *Check
	for i, c := range r.Checks {
		if c.Title == "颜色定义" {
			colorCheck = &r.Checks[i]
		}
	}
	if colorCheck == nil || !strings.Contains(colorCheck.Detail, "5 个颜色定义") {
		t.Fatalf("color count: %+v", r.Checks)
	}
}

func TestValidateMissingSections(t *testing.T) {
	r := Validate([]byte("# Doc\n\n## Overview\n\nOnly overview.\n"))
	if r.HasErrors() {
		t.Fatalf("missing sections must not be errors: %+v", r.Checks)
	}
	warns := levels(r, LevelWarn)
	if len(warns) != 1 || !strings.Contains(warns[0].Detail, "Colors") ||
		!strings.Contains(warns[0].Detail, "Guidelines") {
		t.Fatalf("expected missing-section warning: %+v", warns)
	}
	// overview present -> not listed
	if strings.Contains(warns[0].Detail, "Overview") {
		t.Fatalf("Overview should not be reported missing: %+v", warns)
	}
}

func TestValidateErrors(t *testing.T) {
	// empty
	if r := Validate(nil); !r.HasErrors() {
		t.Fatal("empty should error")
	}
	// non-UTF-8
	r := Validate([]byte("# Title\n\n\x80\x81\x82 binary junk"))
	if !r.HasErrors() {
		t.Fatal("non-UTF-8 should error")
	}
	if r.Checks[0].Level != LevelError || !strings.Contains(r.Checks[0].Title, "UTF-8") {
		t.Fatalf("checks: %+v", r.Checks)
	}
	// oversized
	r = Validate(make([]byte, MaxContentBytes+1))
	if !r.HasErrors() || !strings.Contains(r.Checks[0].Title, "文件过大") {
		t.Fatalf("oversize: %+v", r.Checks)
	}
}

func TestSectionBody(t *testing.T) {
	body := sectionBody(fullDoc, "colors")
	if !strings.Contains(body, "#1a73e8") || strings.Contains(body, "Typography") {
		t.Fatalf("section body: %q", body)
	}
	if sectionBody(fullDoc, "nonexistent") != "" {
		t.Fatal("expected empty body")
	}
	// nested headings stay inside the parent section
	nested := "## Colors\n\nprimary #fff\n\n### Shades\n\nshade #000\n\n## Typography\n"
	body = sectionBody(nested, "colors")
	if !strings.Contains(body, "#000") || strings.Contains(body, "Typography") {
		t.Fatalf("nested section body: %q", body)
	}
}
