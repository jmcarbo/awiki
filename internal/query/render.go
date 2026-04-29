package query

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/dataset"
)

// Render walks all type:query pages and materializes their output datasets.
// Emits QUERY|RENDER|<slug> or QUERY|SKIP|<slug> for each page.
func (r *Runner) Render(ctx context.Context, stdout io.Writer) error {
	queryDir := r.QueryDir()
	entries, err := os.ReadDir(queryDir)
	if os.IsNotExist(err) {
		return nil // no query dir, nothing to do
	}
	if err != nil {
		return err
	}
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		if renderErr := r.RenderOne(ctx, slug, stdout); renderErr != nil && firstErr == nil {
			firstErr = renderErr
		}
	}
	return firstErr
}

// RenderOne renders a single type:query page by slug.
func (r *Runner) RenderOne(ctx context.Context, slug string, stdout io.Writer) error {
	page := filepath.Join(r.QueryDir(), slug+".md")
	pageBytes, err := os.ReadFile(page)
	if err != nil {
		return fmt.Errorf("QUERY|ERROR|no such query: %s", page)
	}
	text := string(pageBytes)

	sql := queryPageSQL(text)
	if sql == "" {
		return fmt.Errorf("QUERY|ERROR|no ```sql fence in %s", page)
	}
	outSlug, _ := dataset.FmGet(text, "out")
	if outSlug == "" {
		return fmt.Errorf("QUERY|ERROR|no out: <slug> in frontmatter of %s", page)
	}

	// Determinism guard.
	if err := IsDeterministic(sql); err != nil {
		return err
	}

	// Materialize via run --out --force.
	opts := RunOptions{SQL: sql, Out: outSlug, Force: true}
	if err := r.Run(ctx, opts, stdout); err != nil {
		return err
	}

	// Compute SQL hash and update page frontmatter + sidecar.
	sqlHash := sqlHashOf(sql)
	newText := updateFMKey(text, "sql_hash", "sha256-"+sqlHash)
	if newText != text {
		os.WriteFile(page, []byte(newText), 0644)
	}

	// Write sidecar .sql.hash file.
	sidecar := filepath.Join(r.QueryDir(), slug+".sql.hash")
	os.WriteFile(sidecar, []byte("sha256-"+sqlHash+"\n"), 0644)

	fmt.Fprintf(stdout, "QUERY|RENDER|%s|out=%s|hash=sha256-%s\n", slug, outSlug, sqlHash)
	return nil
}

// queryPageSQL extracts the SQL from ```sql fence in the page body.
func queryPageSQL(text string) string {
	re := regexp.MustCompile("(?s)```sql\\s*\\n(.*?)\\n```")
	m := re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

// normalizeSQLForHash normalizes SQL for hashing (matching bash cmd_hash).
func normalizeSQLForHash(sql string) string {
	s := strings.Join(strings.Fields(sql), " ")
	return strings.ToLower(strings.TrimSpace(s))
}

// sqlHashOf returns a SHA256 hex hash of normalized SQL.
func sqlHashOf(sql string) string {
	norm := normalizeSQLForHash(sql)
	h := sha256.Sum256([]byte(norm))
	return fmt.Sprintf("%x", h)
}

// updateFMKey replaces or inserts a frontmatter key value.
func updateFMKey(text, key, value string) string {
	return dataset.FmSet(text, key, value)
}
