package initverb

// step_stubs.go holds the last remaining placeholder implementations
// (Step 12 + 13) until their real impls land in the next commit.
// Each stub returns Result{Status: "skipped", Note: "stub"}.

type stepTemplateInit struct{}

func (stepTemplateInit) ID() string                                    { return "template-init" }
func (stepTemplateInit) Description() string                           { return "seed template provenance" }
func (stepTemplateInit) Kind() Kind                                    { return KindMechanical }
func (stepTemplateInit) Execute(StepContext, *Answers) (Result, error) { return stub(), nil }

type stepSmokeTest struct{}

func (stepSmokeTest) ID() string                                    { return "smoke-test" }
func (stepSmokeTest) Description() string                           { return "smoke test prompt" }
func (stepSmokeTest) Kind() Kind                                    { return KindMechanical }
func (stepSmokeTest) Execute(StepContext, *Answers) (Result, error) { return stub(), nil }

func stub() Result { return Result{Status: StatusSkipped, Note: "stub"} }
