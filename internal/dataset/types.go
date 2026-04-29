package dataset

// Storage describes where the dataset content lives.
type Storage string

const (
	StorageInline Storage = "inline"
	StorageFile   Storage = "file"
)

// Format is the data-encoding format of a dataset.
type Format string

const (
	FormatCSV      Format = "csv"
	FormatTSV      Format = "tsv"
	FormatDSV      Format = "dsv"
	FormatJSON     Format = "json"
	FormatTopoJSON Format = "topojson"
)

// Page represents a parsed dataset wiki page.
type Page struct {
	Slug        string
	Path        string
	Storage     Storage
	Format      Format
	Rows        int
	DataPath    string
	Columns     []Column
	Frontmatter map[string]string
}

// Column is a typed column declaration from the dataset frontmatter.
type Column struct {
	Name string
	Type string
}

// ValidationError records a schema-validation failure for a single cell.
type ValidationError struct {
	Row     int
	Column  string
	Want    string
	Got     string
	Missing bool
}
