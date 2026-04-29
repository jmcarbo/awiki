package chart

import (
	"strings"
	"testing"
)

const simplePage = `---
title: "sales"
type: chart
---

# sales

` + "```vega-lite" + `
{"mark": "bar"}
` + "```" + `
`

func TestInjectPreview_InsertWhenAbsent(t *testing.T) {
	result := InjectPreview(simplePage, "sales")
	if !strings.Contains(result, "<!-- BEGIN chart-preview:sales -->") {
		t.Error("expected BEGIN marker")
	}
	if !strings.Contains(result, "<!-- END chart-preview:sales -->") {
		t.Error("expected END marker")
	}
	if !strings.Contains(result, "![sales](../../assets/charts/sales.svg)") {
		t.Errorf("expected image markdown, got:\n%s", result)
	}
}

func TestInjectPreview_RefreshWhenPresent(t *testing.T) {
	// Inject once.
	once := InjectPreview(simplePage, "sales")
	// Inject again (idempotent).
	twice := InjectPreview(once, "sales")
	// Count occurrences: should be exactly 1 BEGIN.
	count := strings.Count(twice, "<!-- BEGIN chart-preview:sales -->")
	if count != 1 {
		t.Errorf("expected exactly 1 BEGIN marker, got %d in:\n%s", count, twice)
	}
}

func TestInjectPreview_ByteExactBody(t *testing.T) {
	result := InjectPreview("# page\n", "my-chart")
	// Verify the body inside the region is exactly the expected image link.
	beginMarker := "<!-- BEGIN chart-preview:my-chart -->\n"
	endMarker := "\n<!-- END chart-preview:my-chart -->"
	start := strings.Index(result, beginMarker)
	if start < 0 {
		t.Fatalf("BEGIN marker not found in:\n%s", result)
	}
	start += len(beginMarker)
	end := strings.Index(result[start:], endMarker)
	if end < 0 {
		t.Fatalf("END marker not found after BEGIN in:\n%s", result)
	}
	body := result[start : start+end]
	want := "![my-chart](../../assets/charts/my-chart.svg)"
	if body != want {
		t.Errorf("body mismatch:\n  got:  %q\n  want: %q", body, want)
	}
}

func TestRemovePreview_WhenAbsent(t *testing.T) {
	result := RemovePreview(simplePage, "sales")
	// Should be unchanged (no region present).
	if result != simplePage {
		t.Errorf("expected no change when region absent:\ngot:\n%s", result)
	}
}

func TestRemovePreview_WhenPresent(t *testing.T) {
	withPreview := InjectPreview(simplePage, "sales")
	result := RemovePreview(withPreview, "sales")
	if strings.Contains(result, "<!-- BEGIN chart-preview:sales -->") {
		t.Errorf("region should be removed:\n%s", result)
	}
	if strings.Contains(result, "![sales]") {
		t.Errorf("image link should be removed:\n%s", result)
	}
}

func TestInjectPreviewAt_RelativePath(t *testing.T) {
	repoRoot := "/repo"
	assetsDir := "/repo/assets/charts"
	pagePath := "/repo/content/charts/sales.md"
	result := InjectPreviewAt("# sales\n", "sales", repoRoot, assetsDir, pagePath)
	// For content/charts/ page, rel to assets/charts is ../../assets/charts/
	if !strings.Contains(result, "../../assets/charts/sales.svg") {
		t.Errorf("expected ../../assets/charts/sales.svg in:\n%s", result)
	}
}
