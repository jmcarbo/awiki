package task

type Level string

const (
	Error Level = "ERROR"
	Warn  Level = "WARN"
	Info  Level = "INFO"
)

type Diagnostic struct {
	Level   Level
	File    string
	Code    string
	Message string
}

type FixRecord struct {
	File    string
	Message string
}

type Options struct {
	RepoRoot   string
	ContentDir string
	Fix        bool
	Today      string
}

type PageInfo struct {
	RelPath     string
	AbsPath     string
	Slug        string
	Type        string
	Status      string
	LastUpdated string
	Aliases     []string
}

type Scan struct {
	Actions       []Action
	Pages         []PageInfo
	Continuations []Continuation
	AgendaEdits   []AgendaEdit
}

type Continuation struct {
	File string
	Line int
}

type AgendaEdit struct {
	File   string
	Line   int
	Region string
}
