package initverb

// DefaultSteps returns the canonical 14-step ordered list. Step
// implementations live in their own files; this constructor only
// concatenates them in the order BOOTSTRAP.md prescribes.
//
// Steps are added incrementally as the package matures. The list
// must always match `bootstrap.ordered_steps` in template.manifest.toml.
func DefaultSteps() []Step {
	return []Step{
		stepDepCheck{},
		stepDomain{},
		stepWikiName{},
		stepPrivacy{},
		stepTrackProcessed{},
		stepTheme{},
		stepPublishLog{},
		stepPatchIdentity{},
		stepInstallQmd{},
		stepWireQmdMCP{},
		stepWireAwikiMCP{},
		stepLogInit{},
		stepStageCommit{},
		stepTemplateInit{},
		stepSmokeTest{},
	}
}
