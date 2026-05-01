package initverb

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ConfirmFn is the yes/no prompt the step impls call. The default
// (defaultValue=false) treats empty input as "n", matching the bash
// `read -r ans; [[ "$ans" =~ ^[Yy] ]]` shape.
type ConfirmFn func(prompt string, defaultValue bool) (bool, error)

// PromptFn is the free-text prompt the step impls call. Empty input
// returns defaultValue.
type PromptFn func(prompt, defaultValue string) (string, error)

// NewPromptFn returns a PromptFn that reads from r and writes the
// prompt to w. The returned closure prints "<prompt> [<default>]: "
// when default is non-empty.
func NewPromptFn(r io.Reader, w io.Writer) PromptFn {
	br := bufio.NewReader(r)
	return func(prompt, def string) (string, error) {
		if def != "" {
			fmt.Fprintf(w, "%s [%s]: ", prompt, def)
		} else {
			fmt.Fprintf(w, "%s: ", prompt)
		}
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return def, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return def, nil
		}
		return line, nil
	}
}

// NewConfirmFn returns a ConfirmFn that reads a single line from r
// and returns true when it begins with y/Y. Empty input returns def.
// The prompt format is "<prompt> (y/N)" or "(Y/n)" depending on def.
func NewConfirmFn(r io.Reader, w io.Writer) ConfirmFn {
	br := bufio.NewReader(r)
	return func(prompt string, def bool) (bool, error) {
		suffix := "(y/N)"
		if def {
			suffix = "(Y/n)"
		}
		fmt.Fprintf(w, "%s %s: ", prompt, suffix)
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return def, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return def, nil
		}
		c := line[0]
		return c == 'y' || c == 'Y', nil
	}
}

// AlwaysDefault returns a PromptFn that ignores stdin and always
// returns the default value. Used in --non-interactive mode.
func AlwaysDefault() PromptFn {
	return func(_, def string) (string, error) {
		return def, nil
	}
}

// AlwaysFalse returns a ConfirmFn that ignores stdin and returns
// the supplied default. Used in --non-interactive mode.
func AlwaysFalse() ConfirmFn {
	return func(_ string, def bool) (bool, error) {
		return def, nil
	}
}
