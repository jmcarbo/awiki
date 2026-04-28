package lint

import "awiki/internal/wiki"

func Run(opts Options) (Collector, int) {
	var c Collector
	if opts.Only != "" && opts.Only != "all" {
		return c, c.ExitCode()
	}

	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{
			Level:   Error,
			File:    opts.ContentDir,
			Message: err.Error(),
		})
		return c, 2
	}

	if opts.Fix {
		applyLastUpdatedFixes(&c, opts, pages)
		pages, err = wiki.DiscoverPages(opts.ContentDir)
		if err != nil {
			c.Add(Diagnostic{
				Level:   Error,
				File:    opts.ContentDir,
				Message: err.Error(),
			})
			return c, 2
		}
	}

	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, idx)
	return c, c.ExitCode()
}
