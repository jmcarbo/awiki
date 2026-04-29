package synth

import "awiki/internal/region"

type GeneratedRegion = region.Generated
type RegionDiagnostic = region.Diagnostic

func ParseGeneratedRegion(text string) (GeneratedRegion, []RegionDiagnostic) {
	return region.ParseGenerated(text)
}
