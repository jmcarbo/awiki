package lint

func Run(opts Options) (Collector, int) {
	var c Collector
	return c, c.ExitCode()
}
