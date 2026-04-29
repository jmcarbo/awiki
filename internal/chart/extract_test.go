package chart

import (
	"testing"
)

const singleFencePage = `---
title: "my-chart"
type: chart
chart_engine: vega-lite
---

# my-chart

` + "```vega-lite" + `
{"mark": "bar", "data": {"name": "[[ds]]"}}
` + "```" + `
`

const multiFencePage = `---
title: "overview"
type: note
---

# overview

` + "```vega-lite" + `
{"mark": "bar"}
` + "```" + `

Some text.

` + "```vega-lite" + `
{"mark": "line"}
` + "```" + `

More text.

` + "```vega-lite" + `
{"mark": "point"}
` + "```" + `
`

const dedicatedChartMultiFence = `---
title: "my-chart"
type: chart
---

` + "```vega-lite" + `
{"mark": "bar"}
` + "```" + `

` + "```vega-lite" + `
{"mark": "line"}
` + "```" + `
`

func TestExtractFences_SingleFromContentPage(t *testing.T) {
	fences := ExtractFences(singleFencePage, "my-chart", true)
	if len(fences) != 1 {
		t.Fatalf("want 1 fence, got %d", len(fences))
	}
	if fences[0].ID != "my-chart" {
		t.Errorf("want ID 'my-chart', got %q", fences[0].ID)
	}
	if fences[0].IsDedicatedChartPage != true {
		t.Error("want IsDedicatedChartPage=true")
	}
}

func TestExtractFences_ThreeWithAutoIDs(t *testing.T) {
	fences := ExtractFences(multiFencePage, "overview", false)
	if len(fences) != 3 {
		t.Fatalf("want 3 fences, got %d", len(fences))
	}
	wantIDs := []string{"overview-fig0", "overview-fig1", "overview-fig2"}
	for i, f := range fences {
		if f.ID != wantIDs[i] {
			t.Errorf("fence %d: want ID %q, got %q", i, wantIDs[i], f.ID)
		}
	}
}

func TestExtractFences_DedicatedChartSingleFence(t *testing.T) {
	// type:chart + single fence → plain slug ID
	page := `---
title: "sales"
type: chart
---

` + "```vega-lite" + `
{"mark": "bar"}
` + "```" + `
`
	fences := ExtractFences(page, "sales", true)
	if len(fences) != 1 {
		t.Fatalf("want 1 fence, got %d", len(fences))
	}
	if fences[0].ID != "sales" {
		t.Errorf("want ID 'sales', got %q", fences[0].ID)
	}
}

func TestExtractFences_DedicatedChartMultiFence(t *testing.T) {
	// type:chart but multiple fences → fig-indexed
	fences := ExtractFences(dedicatedChartMultiFence, "my-chart", true)
	if len(fences) != 2 {
		t.Fatalf("want 2 fences, got %d", len(fences))
	}
	if fences[0].ID != "my-chart-fig0" {
		t.Errorf("want 'my-chart-fig0', got %q", fences[0].ID)
	}
	if fences[1].ID != "my-chart-fig1" {
		t.Errorf("want 'my-chart-fig1', got %q", fences[1].ID)
	}
}

func TestExtractFences_NoFence(t *testing.T) {
	fences := ExtractFences("# hello\n\nno fences here", "page", false)
	if len(fences) != 0 {
		t.Errorf("want 0 fences, got %d", len(fences))
	}
}
