package adapters

// Compile-time conformance: ExecQmd must implement Qmd.
var _ Qmd = (*ExecQmd)(nil)
