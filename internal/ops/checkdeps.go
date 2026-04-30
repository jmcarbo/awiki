package ops

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckDepsOptions configures a single check-deps run. Cases default
// to production (resolve from PATH); tests inject FakeMissing or a
// custom Looker to simulate absent binaries.
type CheckDepsOptions struct {
	OS          string                          // "darwin" / "linux"; defaults to runtime.GOOS-mapped
	RepoRoot    string                          // for .awiki/config + .gitattributes; defaults to cwd
	FakeMissing string                          // matches AWIKI_FAKE_MISSING env var
	BashVersion string                          // "" → auto-detect via "bash --version"
	Looker      func(name string) (bool, error) // PATH lookup; nil → defaults to exec.LookPath
	PyImport    func(mod string) bool           // python module probe; nil → exec(python3 -c)
}

type depCheck struct {
	cmd, minVersion, installMacOS, installLinux string
}

var requiredDeps = []depCheck{
	{"git", "2.30", "brew install git", "apt install git"},
	{"just", "1.13", "brew install just", "cargo install just"},
	{"hugo", "0.120", "brew install hugo", "see https://gohugo.io/installation/"},
	{"bats", "1.10", "brew install bats-core", "apt install bats"},
	{"flock", "n/a",
		"brew install util-linux  # then add the flock binary to PATH (see brew info util-linux)",
		"apt install util-linux  # provides /usr/bin/flock"},
	{"python3", "3.8", "brew install python", "apt install python3"},
}

var optionalCmds = []string{"qmd", "git-crypt", "age", "entr", "fswatch", "inotifywait", "pdftotext"}

// CheckDeps runs the dependency probe with deterministic output. Returns
// the exit code (0 ok, 1 errors). stdout/stderr mirror the bash record
// shapes byte-for-byte.
func CheckDeps(opts CheckDepsOptions, stdout, stderr io.Writer) int {
	osName := opts.OS
	if osName == "" {
		switch runtime.GOOS {
		case "darwin":
			osName = "Darwin"
		case "linux":
			osName = "Linux"
		default:
			osName = strings.Title(runtime.GOOS) //nolint:staticcheck
		}
	}
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	look := opts.Looker
	if look == nil {
		look = defaultLookPath
	}
	pyImport := opts.PyImport
	if pyImport == nil {
		pyImport = defaultPyImport
	}

	errors := 0

	// --- bash version ---
	bv := opts.BashVersion
	if bv == "" {
		bv = detectBashVersion()
	}
	major := bashMajor(bv)
	if major < 4 {
		fmt.Fprintf(stderr, "MISSING|bash|need=4+|have=%s\n", bv)
		switch osName {
		case "Darwin":
			fmt.Fprintln(stderr, "  install: brew install bash")
		case "Linux":
			fmt.Fprintln(stderr, "  install: bash 4+ should be default; check distro packages")
		}
		errors++
	} else {
		fmt.Fprintf(stdout, "OK|bash|%s\n", bv)
	}

	// --- required deps ---
	for _, dep := range requiredDeps {
		if opts.FakeMissing == dep.cmd {
			fmt.Fprintf(stderr, "MISSING|%s|min=%s\n", dep.cmd, dep.minVersion)
			emitInstallHint(stderr, osName, dep)
			errors++
			continue
		}
		ok, _ := look(dep.cmd)
		if !ok {
			fmt.Fprintf(stderr, "MISSING|%s|min=%s\n", dep.cmd, dep.minVersion)
			emitInstallHint(stderr, osName, dep)
			errors++
			continue
		}
		fmt.Fprintf(stdout, "OK|%s\n", dep.cmd)
	}

	// --- optional cmds ---
	for _, cmd := range optionalCmds {
		ok, _ := look(cmd)
		if ok {
			fmt.Fprintf(stdout, "OK|%s (optional)\n", cmd)
		} else {
			fmt.Fprintf(stdout, "OPTIONAL-MISSING|%s\n", cmd)
		}
	}

	// --- expect (with extra hint) ---
	if ok, _ := look("expect"); !ok {
		fmt.Fprintln(stdout, "OPTIONAL-MISSING|expect")
		fmt.Fprintln(stderr, "  warn: 'expect' not installed; interactive triage tests will be skipped.")
		fmt.Fprintln(stderr, "    macOS: brew install expect")
		fmt.Fprintln(stderr, "    debian: apt-get install expect")
	} else {
		fmt.Fprintln(stdout, "OK|expect (optional)")
	}

	// --- pyyaml ---
	if py3, _ := look("python3"); py3 && pyImport("yaml") {
		fmt.Fprintln(stdout, "OK|pyyaml (optional)")
	} else {
		fmt.Fprintln(stdout, "OPTIONAL-MISSING|pyyaml")
		fmt.Fprintln(stderr, "  install: pip3 install pyyaml")
	}

	// --- markdown-it-py ---
	if py3, _ := look("python3"); py3 && pyImport("markdown_it") {
		fmt.Fprintln(stdout, "OK|markdown-it-py (optional)")
	} else {
		fmt.Fprintln(stdout, "OPTIONAL-MISSING|markdown-it-py")
		fmt.Fprintln(stderr, "  install: pip3 install markdown-it-py")
	}

	// --- python-calamine (special FAKE_MISSING token: python_calamine) ---
	switch {
	case opts.FakeMissing == "python_calamine":
		fmt.Fprintln(stdout, "OPTIONAL-MISSING|python-calamine")
		fmt.Fprintln(stderr, "  install: pip3 install python-calamine")
	default:
		py3, _ := look("python3")
		if py3 && pyImport("python_calamine") {
			fmt.Fprintln(stdout, "OK|python-calamine (optional)")
		} else {
			fmt.Fprintln(stdout, "OPTIONAL-MISSING|python-calamine")
			fmt.Fprintln(stderr, "  install: pip3 install python-calamine")
		}
	}

	// --- data-layer advisory ---
	cfgPath := filepath.Join(repoRoot, ".awiki", "config")
	if data, err := os.ReadFile(cfgPath); err == nil {
		if dataLayerOn(string(data)) {
			py3, _ := look("python3")
			if !py3 {
				fmt.Fprintln(stdout, "WARN: data layer is on but python3 is missing — dataset-rows.py won't run")
			}
			vlc, _ := look("vl-convert")
			if !vlc {
				fmt.Fprintln(stdout, "WARN: data layer is on but vl-convert is missing — chart sidecars won't render")
				fmt.Fprintln(stdout, "      install: cargo install vl-convert OR download from https://github.com/vega/vl-convert/releases")
			}
			duck, _ := look("duckdb")
			if opts.FakeMissing == "duckdb" || !duck {
				fmt.Fprintln(stdout, "WARN: data layer is on but duckdb is missing — query layer won't run")
				fmt.Fprintln(stdout, "      install: brew install duckdb (macOS) OR https://duckdb.org/docs/installation/")
			}
		}
	}

	if errors > 0 {
		fmt.Fprintf(stderr, "DEPS-SUMMARY|errors=%d\n", errors)
		return 1
	}
	fmt.Fprintln(stdout, "DEPS-SUMMARY|errors=0")
	return 0
}

func emitInstallHint(stderr io.Writer, osName string, dep depCheck) {
	switch osName {
	case "Darwin":
		fmt.Fprintf(stderr, "  install: %s\n", dep.installMacOS)
	case "Linux":
		fmt.Fprintf(stderr, "  install: %s\n", dep.installLinux)
	}
}

// CheckDepsCLI parses argv and runs the check. AWIKI_FAKE_MISSING env
// var is honoured to mirror the bash form.
func CheckDepsCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki check-deps")
		return 2
	}
	return CheckDeps(CheckDepsOptions{
		FakeMissing: os.Getenv("AWIKI_FAKE_MISSING"),
	}, stdout, stderr)
}

// --- helpers ---

func defaultLookPath(name string) (bool, error) {
	_, err := exec.LookPath(name)
	return err == nil, err
}

func defaultPyImport(mod string) bool {
	cmd := exec.Command("python3", "-c", "import "+mod)
	return cmd.Run() == nil
}

func detectBashVersion() string {
	if v := os.Getenv("BASH_VERSION"); v != "" {
		return v
	}
	out, err := exec.Command("bash", "--version").Output()
	if err != nil {
		return ""
	}
	// First line: "GNU bash, version 5.2.21(1)-release (x86_64-apple-darwin)"
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	if scanner.Scan() {
		line := scanner.Text()
		_, after, ok := strings.Cut(line, "version ")
		if !ok {
			return ""
		}
		// Trim at first space.
		end := strings.IndexByte(after, ' ')
		if end < 0 {
			return after
		}
		return after[:end]
	}
	return ""
}

func bashMajor(v string) int {
	if v == "" {
		return 0
	}
	maj, _, _ := strings.Cut(v, ".")
	n := 0
	for _, c := range maj {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func dataLayerOn(cfg string) bool {
	for line := range strings.SplitSeq(cfg, "\n") {
		line = strings.TrimSpace(line)
		if line == "AWIKI_DATA_LAYER=on" {
			return true
		}
	}
	return false
}
