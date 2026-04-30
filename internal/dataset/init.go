package dataset

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DataInit runs all idempotent data-layer initialisation steps. The
// interactive encryption step is NOT ported; a record is emitted to stderr
// to instruct the user to run the bash script for that step.
func (r *Runner) DataInit(stdout, stderr io.Writer) error {
	if err := r.ensureDirs(stdout); err != nil {
		return err
	}
	if err := r.ensureVegaVendor(stdout); err != nil {
		return err
	}
	if err := r.ensureWikiMd(stdout, stderr); err != nil {
		return err
	}
	if err := r.ensureConfig(stdout); err != nil {
		return err
	}
	fmt.Fprintln(stderr, "DATA-INIT|encryption-skipped|run 'awiki encrypt-init' (TODO) or 'bash scripts/encrypt-init.sh' for interactive setup")
	return nil
}

// ensureDirs creates the required data-layer directories with .gitkeep files.
// Matches step_dirs in scripts/data-init.sh.
func (r *Runner) ensureDirs(stdout io.Writer) error {
	dirs := []string{
		"content/datasets",
		"content/queries",
		"content/charts",
		"data",
		"assets/charts",
		"static/vendor/vega",
	}
	for _, d := range dirs {
		full := filepath.Join(r.RepoRoot, d)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
			gk := filepath.Join(full, ".gitkeep")
			if err := os.WriteFile(gk, nil, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "DATA-INIT|created %s\n", d)
		} else {
			fmt.Fprintf(stdout, "DATA-INIT|skip %s (exists)\n", d)
			// Ensure .gitkeep exists even if dir was already there.
			gk := filepath.Join(full, ".gitkeep")
			if _, err := os.Stat(gk); os.IsNotExist(err) {
				if err := os.WriteFile(gk, nil, 0o644); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "DATA-INIT|restored %s/.gitkeep\n", d)
			}
		}
	}
	return nil
}

// ensureVegaVendor shells to vendor-vega.sh if any of the expected vega files
// are missing. Matches step_vendor_vega in scripts/data-init.sh.
func (r *Runner) ensureVegaVendor(stdout io.Writer) error {
	vendorDir := filepath.Join(r.RepoRoot, "static", "vendor", "vega")
	expected := []string{
		"vega.min.js",
		"vega-lite.min.js",
		"vega-embed.min.js",
	}
	allPresent := true
	for _, f := range expected {
		if _, err := os.Stat(filepath.Join(vendorDir, f)); os.IsNotExist(err) {
			allPresent = false
			break
		}
	}
	if allPresent {
		fmt.Fprintln(stdout, "DATA-INIT|skip vega vendor (all files present)")
		return nil
	}
	vendorScript := filepath.Join(r.RepoRoot, "scripts", "lib", "vendor-vega.sh")
	cmd := exec.Command("bash", vendorScript, vendorDir)
	cmd.Dir = r.RepoRoot
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Run(); err != nil {
		// Matches bash: warn "vega vendor failed" — non-fatal.
		fmt.Fprintf(stdout, "DATA-INIT|WARN|vega vendor failed\n")
	}
	return nil
}

// ensureWikiMd patches WIKI.md with the data-layer managed region.
// Matches step_wiki_md in scripts/data-init.sh.
func (r *Runner) ensureWikiMd(stdout, stderr io.Writer) error {
	templatePath := filepath.Join(r.RepoRoot, "scripts", "templates", "wiki-data-layer.md")
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintf(stderr, "DATA-INIT|WARN|missing %s — cannot patch WIKI.md\n", templatePath)
		return nil
	}
	tmpl := string(tmplBytes)

	wikiPath := filepath.Join(r.RepoRoot, "WIKI.md")
	if _, err := os.Stat(wikiPath); os.IsNotExist(err) {
		fmt.Fprintln(stderr, "DATA-INIT|WARN|no WIKI.md found — skipping patch")
		return nil
	}
	wikiBytes, err := os.ReadFile(wikiPath)
	if err != nil {
		return err
	}
	wikiText := string(wikiBytes)

	begin := "<!-- BEGIN data-layer -->"
	end := "<!-- END data-layer -->"

	if strings.Contains(wikiText, begin) {
		if !strings.Contains(wikiText, end) {
			fmt.Fprintf(stderr, "DATA-INIT|WARN|WIKI.md has BEGIN marker but no END marker — refusing to patch\n")
			return nil
		}
		// Replace existing block (idempotent refresh).
		newText := replaceDataLayerBlock(wikiText, begin, end, tmpl)
		if err := os.WriteFile(wikiPath, []byte(newText), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "DATA-INIT|refreshed data-layer block in WIKI.md")
	} else {
		// Append the template to WIKI.md.
		f, err := os.OpenFile(wikiPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintf(f, "\n%s", tmpl)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintln(stdout, "DATA-INIT|appended data-layer block to WIKI.md")
	}
	return nil
}

// replaceDataLayerBlock replaces the content between begin and end markers
// with the template string. Matches the awk logic in step_wiki_md.
func replaceDataLayerBlock(text, begin, end, tmpl string) string {
	var out strings.Builder
	inBlock := false
	replaced := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if !replaced && strings.Contains(line, begin) {
			// Emit the template instead.
			out.WriteString(tmpl)
			if !strings.HasSuffix(tmpl, "\n") {
				out.WriteString("\n")
			}
			inBlock = true
			replaced = true
			continue
		}
		if inBlock && strings.Contains(line, end) {
			inBlock = false
			continue
		}
		if !inBlock {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}
	result := out.String()
	// Remove trailing extra newline if original didn't end with one.
	if !strings.HasSuffix(text, "\n") && strings.HasSuffix(result, "\n") {
		result = strings.TrimRight(result, "\n")
	}
	return result
}

// ensureConfig appends missing key/value pairs to .awiki/config.
// Matches step_config / _ensure_kv in scripts/data-init.sh.
func (r *Runner) ensureConfig(stdout io.Writer) error {
	awikiDir := filepath.Join(r.RepoRoot, ".awiki")
	if err := os.MkdirAll(awikiDir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(awikiDir, "config")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, nil, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "DATA-INIT|created %s\n", cfgPath)
	}

	defaults := []struct{ key, val string }{
		{"AWIKI_DATA_LAYER", "on"},
		{"AWIKI_DATASET_INLINE_MAX_ROWS", "500"},
		{"AWIKI_DATASET_INLINE_MAX_BYTES", "51200"},
		{"AWIKI_CHART_OBSIDIAN_PREVIEW", "on"},
		{"AWIKI_QUERY_LAYER", "on"},
	}
	for _, kv := range defaults {
		if err := r.ensureKV(cfgPath, kv.key, kv.val, stdout); err != nil {
			return err
		}
	}
	return nil
}

// ensureKV appends key=val to the config file only when the key is absent.
func (r *Runner) ensureKV(cfgPath, key, val string, stdout io.Writer) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			fmt.Fprintf(stdout, "DATA-INIT|skip %s (already set)\n", key)
			return nil
		}
	}
	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintf(f, "%s=%s\n", key, val)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Fprintf(stdout, "DATA-INIT|appended %s=%s\n", key, val)
	return nil
}
