package lint

import "awiki/internal/wiki"

func Run(opts Options) (Collector, int) {
	var c Collector
	pages, err := wiki.DiscoverPages(opts.ContentDir)
	if err != nil {
		c.Add(Diagnostic{
			Level:   Error,
			File:    opts.ContentDir,
			Message: err.Error(),
		})
		return c, 2
	}

	idx := wiki.BuildIndex(pages)
	runCoreRules(&c, idx)
	return c, c.ExitCode()
}
