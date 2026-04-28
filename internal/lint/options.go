package lint

import "awiki/internal/adapters"

type Options struct {
	Fix            bool
	Only           string
	OnlyFile       string
	HugoCheck      bool
	AliasBuildOnly bool
	ContentDir     string
	RepoRoot       string
	Today          string
	Runner         adapters.Runner
}
