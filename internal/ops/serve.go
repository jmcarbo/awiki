package ops

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"awiki/internal/adapters"
)

// ServeOptions configures `awiki serve`. Mirrors scripts/serve.sh.
type ServeOptions struct {
	RepoRoot string
	Bind     string
	Port     int
	// HugoRunner exec's `hugo serve`; defaults to adapters.ExecRunner{}.
	HugoRunner adapters.Runner
	// SkipWatcher disables the background rebuild loop (tests).
	SkipWatcher bool
	// SkipHugo disables the foreground hugo serve (tests).
	SkipHugo bool
}

// Serve runs the local hugo dev server. Side-effect summary:
//
//  1. Clear `resources/_gen`, `public/`, `.hugo_build.lock` (stale fingerprint
//     guard from scripts/serve.sh).
//  2. One-shot Build (no flags).
//  3. Background watcher that re-runs Build on content/ changes (mtime poll
//     1Hz; mirrors the bash poll-mtime branch — entr/fswatch not honoured).
//  4. Foreground `hugo server --bind <Bind> --port <Port> -D`.
func Serve(opts ServeOptions, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Bind == "" {
		opts.Bind = "0.0.0.0"
	}
	if opts.Port == 0 {
		opts.Port = 1313
	}
	if opts.HugoRunner == nil {
		opts.HugoRunner = adapters.ExecRunner{}
	}
	// Clear stale Hugo state.
	for _, p := range []string{"resources/_gen", "public", ".hugo_build.lock"} {
		_ = os.RemoveAll(filepath.Join(opts.RepoRoot, p))
	}

	// One-shot build.
	if rc := Build(BuildOptions{RepoRoot: opts.RepoRoot}, stdout, stderr); rc != 0 {
		return rc
	}

	if opts.SkipWatcher && opts.SkipHugo {
		return 0
	}

	stop := make(chan struct{})
	if !opts.SkipWatcher {
		fmt.Fprintln(stdout, "WATCH|tool=poll-mtime")
		fmt.Fprintln(stdout, "  (install fswatch or entr for inotify-style change detection)")
		go pollContentLoop(opts.RepoRoot, stop, stdout, stderr)
	}

	if opts.SkipHugo {
		return 0
	}

	// Foreground hugo serve. Block until SIGINT/TERM or hugo exits.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hugo", "server",
		"--bind", opts.Bind, "--port", strconv.Itoa(opts.Port), "-D")
	cmd.Dir = opts.RepoRoot
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	close(stop)
	if err != nil {
		// SIGINT propagates as exec.ExitError; treat clean shutdown as 0.
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "hugo serve: %v\n", err)
		return 1
	}
	return 0
}

// pollContentLoop rebuilds when content/ mtimes change (1s poll). Stops
// when <stop> closes.
func pollContentLoop(repoRoot string, stop <-chan struct{}, stdout, stderr io.Writer) {
	contentDir := filepath.Join(repoRoot, "content")
	prev := contentSnapshot(contentDir)
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			cur := contentSnapshot(contentDir)
			if cur != prev {
				_ = Build(BuildOptions{RepoRoot: repoRoot}, stdout, stderr)
				prev = cur
			}
		}
	}
}

// contentSnapshot returns a single string mixing per-file mtime+path, used as
// a cheap change-detection fingerprint.
func contentSnapshot(dir string) string {
	var b []byte
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".md" {
			return nil
		}
		b = fmt.Appendf(b, "%d %s\n", info.ModTime().UnixNano(), p)
		return nil
	})
	return string(b)
}

// ServeCLI parses argv and dispatches Serve.
func ServeCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 1313, "port to listen on")
	bind := fs.String("bind", "0.0.0.0", "interface to bind")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return Serve(ServeOptions{
		RepoRoot: repoRoot,
		Port:     *port,
		Bind:     *bind,
	}, stdout, stderr)
}
