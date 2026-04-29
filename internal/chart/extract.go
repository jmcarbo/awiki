package chart

import (
	"fmt"
	"regexp"
	"strings"
)

var vegaLiteFenceRE = regexp.MustCompile("(?s)```vega-lite\\s*\\n(.*?)\\n```")

// ExtractFences walks pageText for ```vega-lite ... ``` blocks and returns
// a ChartFence for each one.
//
// Chart ID assignment matches the bash _extract_fences helper:
//   - dedicated type:chart page with exactly one fence → ID is slug
//   - otherwise → ID is slug-fig<idx> (0-based index matching bash behaviour)
func ExtractFences(pageText, slug string, isDedicated bool) []ChartFence {
	matches := vegaLiteFenceRE.FindAllStringSubmatch(pageText, -1)
	if len(matches) == 0 {
		return nil
	}
	fences := make([]ChartFence, 0, len(matches))
	single := isDedicated && len(matches) == 1
	for idx, m := range matches {
		id := slug
		if !single {
			id = fmt.Sprintf("%s-fig%d", slug, idx)
		}
		fences = append(fences, ChartFence{
			ID:                   id,
			Spec:                 []byte(strings.TrimSpace(m[1])),
			PageSlug:             slug,
			Index:                idx,
			IsDedicatedChartPage: single,
		})
	}
	return fences
}
