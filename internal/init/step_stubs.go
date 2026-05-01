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
