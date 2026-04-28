package query

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

type Options struct {
	RepoRoot   string
	ContentDir string
	OnlyFile   string
}
