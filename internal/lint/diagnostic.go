package lint

import "fmt"

type Level string

const (
	Error Level = "ERROR"
	Warn  Level = "WARN"
	Info  Level = "INFO"
)

type Diagnostic struct {
	Level   Level
	File    string
	Message string
}

func (d Diagnostic) Record() string {
	return fmt.Sprintf("LINT|%s|%s|%s", d.Level, d.File, d.Message)
}

type FixRecord struct {
	File    string
	Message string
}

func (r FixRecord) Record() string {
	return fmt.Sprintf("FIX|%s|%s", r.File, r.Message)
}

type Collector struct {
	Diagnostics []Diagnostic
	Fixes       []FixRecord
}

func (c *Collector) Add(d Diagnostic) {
	c.Diagnostics = append(c.Diagnostics, d)
}

func (c *Collector) AddFix(f FixRecord) {
	c.Fixes = append(c.Fixes, f)
}

func (c Collector) Counts() (errors int, warnings int, infos int) {
	for _, d := range c.Diagnostics {
		switch d.Level {
		case Error:
			errors++
		case Warn:
			warnings++
		case Info:
			infos++
		}
	}
	return errors, warnings, infos
}

func (c Collector) Summary() string {
	errors, warnings, infos := c.Counts()
	return fmt.Sprintf("LINT-SUMMARY|errors=%d|warnings=%d|info=%d", errors, warnings, infos)
}

func (c Collector) ExitCode() int {
	errors, warnings, _ := c.Counts()
	if errors > 0 {
		return 2
	}
	if warnings > 0 {
		return 1
	}
	return 0
}
