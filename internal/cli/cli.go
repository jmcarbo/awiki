package cli

import (
	"fmt"
	"io"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki <command> [args]")
		return 1
	}
	switch args[0] {
	case "lint":
		fmt.Fprintln(stderr, "awiki lint is not implemented yet")
		return 1
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}
