package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LsRemoteFunc resolves the upstream tip SHA for repo+ref. The function
// returns the leading SHA from `git ls-remote <repo> <ref>` or an empty
// string with a non-nil error on any failure (network, missing binary,
// timeout). The Python oracle silently swallows all failures, so
// implementations are expected to do the same and return ("", nil) when
// they want the caller to skip the availability check rather than
// surface an error.
type LsRemoteFunc func(repo, ref string) (sha string, err error)

// AvailabilityResult captures the side-effects CheckAvailability would
// otherwise produce on stdout. The Python oracle prints a single
// "LINT|info|template|..." line when an update is available; we hand
// that line back to the caller so the CLI driver decides what to do
// with it (typically `fmt.Fprintln(stdout, msg)`).
type AvailabilityResult struct {
	// Skipped reports whether the check short-circuited (env var set,
	// config opt-out, missing pin, fresh stamp, etc.).
	Skipped bool
	// Message is non-empty iff there is an update available and a
	// stamp refresh succeeded. Mirrors the lone `print(...)` in the
	// Python oracle.
	Message string
}

// CheckAvailability mirrors `availability.py --check`. It returns an
// AvailabilityResult; never returns an error for the common skip cases
// (network failure, missing pin, opt-out) — those return Skipped=true
// with no message. Only filesystem errors that block writing the stamp
// or reading the provenance get surfaced.
//
// noCheckEnv is the value of AWIKI_NO_TEMPLATE_CHECK ("1" disables).
// now is the wall clock used to compare against the cached stamp mtime
// — pass time.Now to use real time.
func CheckAvailability(root string, noCheckEnv string, now time.Time, lsRemote LsRemoteFunc) (AvailabilityResult, error) {
	if noCheckEnv == "1" {
		return AvailabilityResult{Skipped: true}, nil
	}
	pj := filepath.Join(root, ".awiki", "template.json")
	if !isFile(pj) {
		return AvailabilityResult{Skipped: true}, nil
	}
	prov, err := LoadProvenance(pj)
	if err != nil {
		// Python catches every exception and returns 0; mirror that.
		return AvailabilityResult{Skipped: true}, nil
	}

	configPath := filepath.Join(root, ".awiki", "config")
	if isFile(configPath) {
		data, err := os.ReadFile(configPath)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) == "no_template_check=true" {
					return AvailabilityResult{Skipped: true}, nil
				}
			}
		}
	}

	cacheDir := filepath.Join(root, ".awiki", "template-cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return AvailabilityResult{}, err
	}
	stamp := filepath.Join(cacheDir, "_check-stamp")
	cutoff := now.UTC().Add(-7 * 24 * time.Hour)
	if fi, err := os.Stat(stamp); err == nil {
		// Match Python: fromtimestamp(stat.st_mtime, tz=UTC) > cutoff
		// means "fresh" — short-circuit.
		if fi.ModTime().UTC().After(cutoff) {
			return AvailabilityResult{Skipped: true}, nil
		}
	}

	repo := prov.Repo
	ref := prov.Ref
	if ref == "" {
		ref = "main"
	}
	pinned := prov.Commit
	if repo == "" {
		return AvailabilityResult{Skipped: true}, nil
	}
	if lsRemote == nil {
		// No way to reach upstream — match Python's "missing-git" path.
		return AvailabilityResult{Skipped: true}, nil
	}
	upstream, err := lsRemote(repo, ref)
	if err != nil || upstream == "" {
		return AvailabilityResult{Skipped: true}, nil
	}

	// Touch the stamp regardless of outcome. Python wraps in try/except
	// and silently ignores; we do the same.
	_ = touchFile(stamp, now)

	if upstream != pinned {
		short := pinned
		if len(short) > 12 {
			short = short[:12]
		}
		msg := fmt.Sprintf(
			"LINT|info|template|template updates available since %s. Run `just template-status` to review.",
			short,
		)
		return AvailabilityResult{Message: msg}, nil
	}
	return AvailabilityResult{}, nil
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !fi.IsDir()
}

// touchFile creates path with empty contents if missing, otherwise
// updates its mtime to now. Mirrors `Path.touch()`.
func touchFile(path string, now time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_ = f.Close()
	}
	return os.Chtimes(path, now, now)
}
