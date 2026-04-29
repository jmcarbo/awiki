package adapters

// Compile-time assertion: ExecDuckDB must implement the DuckDB interface.
var _ DuckDB = (*ExecDuckDB)(nil)
