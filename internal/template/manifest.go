package template

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// strategyPrecedence mirrors STRATEGY_PRECEDENCE in the Python oracle
// (scripts/_template_helpers/manifest_parse.py). Earlier entries win
// cross-strategy ties at the same glob specificity.
var strategyPrecedence = []string{
	"template_only",
	"attributes_merge",
	"three_way",
	"overwrite",
	"preserve",
}

// Manifest is the parsed template.manifest.toml document.
type Manifest struct {
	SchemaVersion   int                 `toml:"schema_version"`
	TemplateVersion string              `toml:"template_version"`
	Strategies      map[string][]string `toml:"strategies"`
	NewFileDefault  NewFileDefault      `toml:"new_file_default"`
	Bootstrap       Bootstrap           `toml:"bootstrap"`
}

// NewFileDefault is the fallback strategy declaration for files that
// match no glob in any strategy bucket.
type NewFileDefault struct {
	Strategy string `toml:"strategy"`
}

// Bootstrap captures bootstrap-step ordering and the dangerous-id set.
type Bootstrap struct {
	OrderedSteps []string            `toml:"ordered_steps"`
	Dangerous    BootstrapDangerous  `toml:"dangerous"`
}

// BootstrapDangerous lists step IDs that require explicit user
// confirmation before replay.
type BootstrapDangerous struct {
	IDs []string `toml:"ids"`
}

// LoadManifest reads and parses the manifest at path. It returns a
// distinguishable error when the file is missing.
func LoadManifest(p string) (*Manifest, error) {
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("manifest not found: %s", p)
		}
		return nil, err
	}
	var m Manifest
	if _, err := toml.DecodeFile(p, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", p, err)
	}
	if m.NewFileDefault.Strategy == "" {
		// Mirror the Python `m.get("new_file_default", {}).get("strategy", "prompt")`
		// fallback so callers that read NewFileDefault.Strategy directly
		// observe the same default the resolver uses.
		m.NewFileDefault.Strategy = "prompt"
	}
	return &m, nil
}

// FormatLoad returns the shell-sourceable KEY=value record produced by
// `manifest_parse.py load`. The output ends with a trailing newline.
func (m *Manifest) FormatLoad() string {
	nfd := m.NewFileDefault.Strategy
	if nfd == "" {
		nfd = "prompt"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "schema_version=%d\n", m.SchemaVersion)
	fmt.Fprintf(&b, "template_version=%s\n", m.TemplateVersion)
	fmt.Fprintf(&b, "new_file_default=%s\n", nfd)
	return b.String()
}

// ResolveStrategy returns the strategy that applies to relpath, using
// the same most-specific-glob-wins logic as the Python oracle.
func (m *Manifest) ResolveStrategy(relpath string) string {
	type cand struct {
		spec       globSpec
		strategy   string
		precedence int
		idx        int
	}
	var cs []cand
	for precIdx, strat := range strategyPrecedence {
		globs := m.Strategies[strat]
		for i, g := range globs {
			if globMatches(g, relpath) {
				cs = append(cs, cand{
					spec:       glob_specificity(g),
					strategy:   strat,
					precedence: precIdx,
					idx:        i,
				})
			}
		}
	}
	if len(cs) == 0 {
		nfd := m.NewFileDefault.Strategy
		if nfd == "" {
			return "prompt"
		}
		return nfd
	}
	// Sort: highest specificity first; ties broken by precedence
	// position (lower idx = higher priority); final fallback: later-
	// declared (higher idx) wins. We sort descending by:
	//   spec, -precedence, idx
	sort.SliceStable(cs, func(i, j int) bool {
		if c := cs[i].spec.compare(cs[j].spec); c != 0 {
			return c > 0
		}
		if cs[i].precedence != cs[j].precedence {
			return cs[i].precedence < cs[j].precedence
		}
		return cs[i].idx > cs[j].idx
	})
	return cs[0].strategy
}

// HasGlobOverlap returns the first within-strategy duplicate glob it
// finds (along with the strategy name), or "", "", nil if there are
// none.
func (m *Manifest) HasGlobOverlap() (strategy, glob string, ok bool) {
	// Iterate strategies in precedence order so the error message
	// matches the Python oracle's stable order. The Python helper
	// iterates `m.get("strategies", {}).items()` which is dict-insertion
	// order — but tomllib also preserves insertion order, and
	// Python's dict iteration mirrors that. Since map ordering in Go
	// is random, we iterate the precedence list (the canonical set
	// of buckets) plus any extras in lex order for determinism.
	seenStrats := map[string]bool{}
	walk := func(strat string) (string, string, bool) {
		globs := m.Strategies[strat]
		seen := map[string]bool{}
		for _, g := range globs {
			if seen[g] {
				return strat, g, true
			}
			seen[g] = true
		}
		return "", "", false
	}
	for _, s := range strategyPrecedence {
		seenStrats[s] = true
		if strat, g, ok := walk(s); ok {
			return strat, g, true
		}
	}
	// Walk any other buckets in sorted order for determinism.
	var extra []string
	for s := range m.Strategies {
		if !seenStrats[s] {
			extra = append(extra, s)
		}
	}
	sort.Strings(extra)
	for _, s := range extra {
		if strat, g, ok := walk(s); ok {
			return strat, g, true
		}
	}
	return "", "", false
}

// globSpec mirrors the `_glob_specificity` tuple in the Python oracle.
type globSpec struct {
	literalPrefix int
	segments      int
	noDoubleStar  int
	glob          string
}

func glob_specificity(g string) globSpec {
	starIdx := strings.Index(g, "*")
	literalPrefix := len(g)
	if starIdx >= 0 {
		literalPrefix = starIdx
	}
	segments := strings.Count(g, "/") + 1
	noDoubleStar := 1
	if strings.Contains(g, "**") {
		noDoubleStar = 0
	}
	return globSpec{literalPrefix, segments, noDoubleStar, g}
}

// compare returns >0 if a is more specific than b, <0 if less, 0 if
// equal. It matches Python tuple comparison `(literal, segments,
// noDoubleStar, glob)`.
func (a globSpec) compare(b globSpec) int {
	if a.literalPrefix != b.literalPrefix {
		if a.literalPrefix > b.literalPrefix {
			return 1
		}
		return -1
	}
	if a.segments != b.segments {
		if a.segments > b.segments {
			return 1
		}
		return -1
	}
	if a.noDoubleStar != b.noDoubleStar {
		if a.noDoubleStar > b.noDoubleStar {
			return 1
		}
		return -1
	}
	return strings.Compare(a.glob, b.glob)
}

// globMatches mirrors `_matches` in the Python oracle. fnmatch with
// `**` expanded to "anything spanning path segments".
func globMatches(g, relpath string) bool {
	if strings.Contains(g, "**") {
		parts := strings.SplitN(g, "**", 2)
		if len(parts) == 2 {
			prefix := strings.TrimRight(parts[0], "/")
			suffix := strings.TrimLeft(parts[1], "/")
			if prefix != "" {
				if !(relpath == prefix || strings.HasPrefix(relpath, prefix+"/")) {
					return false
				}
			}
			if suffix != "" {
				// Mirror Python: relpath == suffix OR relpath ends with
				// "/" + suffix OR fnmatch.fnmatch(relpath, "*"+suffix).
				if relpath == suffix {
					return true
				}
				if strings.HasSuffix(relpath, "/"+suffix) {
					return true
				}
				if fnmatch("*"+suffix, relpath) {
					return true
				}
				return false
			}
			return true
		}
	}
	return fnmatch(g, relpath)
}

// fnmatch ports Python's fnmatch.fnmatch semantics. `*` matches any
// sequence of characters (including `/`), `?` matches a single
// character, `[seq]` matches a character class, and `[!seq]` is its
// negation. Unlike Go's path.Match, `*` here may span path separators.
func fnmatch(pattern, name string) bool {
	re, err := regexp.Compile("^" + fnmatchToRegex(pattern) + "$")
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// fnmatchToRegex translates an fnmatch pattern to a regex source. The
// translation matches CPython's fnmatch.translate for the subset of
// patterns the manifest actually uses.
func fnmatchToRegex(p string) string {
	var b strings.Builder
	i := 0
	n := len(p)
	for i < n {
		c := p[i]
		i++
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		case '[':
			// Find the closing ']'.
			j := i
			if j < n && p[j] == '!' {
				j++
			}
			if j < n && p[j] == ']' {
				j++
			}
			for j < n && p[j] != ']' {
				j++
			}
			if j >= n {
				// No closing bracket; treat as literal.
				b.WriteString(regexp.QuoteMeta("["))
			} else {
				class := p[i:j]
				i = j + 1
				if strings.HasPrefix(class, "!") {
					class = "^" + class[1:]
				}
				b.WriteByte('[')
				b.WriteString(class)
				b.WriteByte(']')
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
