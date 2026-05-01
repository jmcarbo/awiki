package initverb

// step_stubs.go holds placeholder implementations for the steps that
// will land in subsequent commits. Each stub returns Result{Status:
// "skipped", Note: "stub"} so the orchestrator wiring can compile +
// be exercised by the scaffolding tests before the real impls land.
//
// As each real impl moves into its own file (e.g. step_depcheck.go,
// step_domain.go), the corresponding stub here is deleted. When
// step_stubs.go is empty it gets removed.

// All stubs intentionally have zero size so they cost nothing at
// runtime. They're declared in dependency order (Step 0 → Step 13).

type stepDepCheck struct{}

func (stepDepCheck) ID() string                                       { return "dep-check" }
func (stepDepCheck) Description() string                              { return "submodules + dependency check" }
func (stepDepCheck) Kind() Kind                                       { return KindMechanical }
func (stepDepCheck) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepDomain struct{}

func (stepDomain) ID() string                                       { return "domain" }
func (stepDomain) Description() string                              { return "pick wiki domain" }
func (stepDomain) Kind() Kind                                       { return KindInteractive }
func (stepDomain) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepWikiName struct{}

func (stepWikiName) ID() string                                       { return "wiki-name" }
func (stepWikiName) Description() string                              { return "wiki name + purpose" }
func (stepWikiName) Kind() Kind                                       { return KindInteractive }
func (stepWikiName) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepPrivacy struct{}

func (stepPrivacy) ID() string                                       { return "privacy" }
func (stepPrivacy) Description() string                              { return "encryption decision" }
func (stepPrivacy) Kind() Kind                                       { return KindHybrid }
func (stepPrivacy) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepTrackProcessed struct{}

func (stepTrackProcessed) ID() string                                       { return "track-processed" }
func (stepTrackProcessed) Description() string                              { return "track ingested sources?" }
func (stepTrackProcessed) Kind() Kind                                       { return KindHybrid }
func (stepTrackProcessed) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepTheme struct{}

func (stepTheme) ID() string                                       { return "theme" }
func (stepTheme) Description() string                              { return "Hugo theme" }
func (stepTheme) Kind() Kind                                       { return KindHybrid }
func (stepTheme) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepPublishLog struct{}

func (stepPublishLog) ID() string                                       { return "publish-log" }
func (stepPublishLog) Description() string                              { return "publish log to rendered site?" }
func (stepPublishLog) Kind() Kind                                       { return KindHybrid }
func (stepPublishLog) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepPatchIdentity struct{}

func (stepPatchIdentity) ID() string                                       { return "patch-identity" }
func (stepPatchIdentity) Description() string                              { return "patch identity files" }
func (stepPatchIdentity) Kind() Kind                                       { return KindHybrid }
func (stepPatchIdentity) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepInstallQmd struct{}

func (stepInstallQmd) ID() string                                       { return "install-qmd" }
func (stepInstallQmd) Description() string                              { return "install qmd" }
func (stepInstallQmd) Kind() Kind                                       { return KindMechanical }
func (stepInstallQmd) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepWireQmdMCP struct{}

func (stepWireQmdMCP) ID() string                                       { return "wire-qmd-mcp" }
func (stepWireQmdMCP) Description() string                              { return "wire qmd MCP server" }
func (stepWireQmdMCP) Kind() Kind                                       { return KindHybrid }
func (stepWireQmdMCP) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepWireAwikiMCP struct{}

func (stepWireAwikiMCP) ID() string                                       { return "wire-awiki-mcp" }
func (stepWireAwikiMCP) Description() string                              { return "wire awiki MCP server" }
func (stepWireAwikiMCP) Kind() Kind                                       { return KindHybrid }
func (stepWireAwikiMCP) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepLogInit struct{}

func (stepLogInit) ID() string                                       { return "log-init" }
func (stepLogInit) Description() string                              { return "log init entry" }
func (stepLogInit) Kind() Kind                                       { return KindMechanical }
func (stepLogInit) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepStageCommit struct{}

func (stepStageCommit) ID() string                                       { return "stage-commit" }
func (stepStageCommit) Description() string                              { return "initial commit" }
func (stepStageCommit) Kind() Kind                                       { return KindHybrid }
func (stepStageCommit) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepTemplateInit struct{}

func (stepTemplateInit) ID() string                                       { return "template-init" }
func (stepTemplateInit) Description() string                              { return "seed template provenance" }
func (stepTemplateInit) Kind() Kind                                       { return KindMechanical }
func (stepTemplateInit) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

type stepSmokeTest struct{}

func (stepSmokeTest) ID() string                                       { return "smoke-test" }
func (stepSmokeTest) Description() string                              { return "smoke test prompt" }
func (stepSmokeTest) Kind() Kind                                       { return KindMechanical }
func (stepSmokeTest) Execute(StepContext, *Answers) (Result, error)    { return stub(), nil }

func stub() Result { return Result{Status: StatusSkipped, Note: "stub"} }
