package lint

import (
	"fmt"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

func runCoreRules(c *Collector, idx wiki.Index) {
	addDuplicateSlugDiagnostics(c, idx)
	addDuplicateAliasDiagnostics(c, idx)

	for _, page := range idx.Pages {
		addEmptyPageDiagnostic(c, page)
		addPrivateTagDiagnostic(c, page)
		addBrokenWikilinkDiagnostics(c, idx, page)
	}

	addOrphanDiagnostics(c, idx)
	addCatalogDiagnostics(c, idx)
}

func addDuplicateSlugDiagnostics(c *Collector, idx wiki.Index) {
	slugs := sortedKeys(idx.DuplicateSlugs)
	for _, slug := range slugs {
		pages := idx.DuplicateSlugs[slug]
		if len(pages) < 2 {
			continue
		}
		first := pages[0]
		for _, page := range pages[1:] {
			c.Add(Diagnostic{
				Level:   Error,
				File:    page.Path,
				Message: fmt.Sprintf("duplicate slug: %s also at %s", slug, first.Path),
			})
		}
	}
}

func addDuplicateAliasDiagnostics(c *Collector, idx wiki.Index) {
	aliases := sortedKeys(idx.AliasCollision)
	for _, alias := range aliases {
		for _, page := range idx.AliasCollision[alias] {
			c.Add(Diagnostic{
				Level:   Error,
				File:    page.Path,
				Message: fmt.Sprintf("duplicate alias: %s", alias),
			})
		}
	}
}

func addEmptyPageDiagnostic(c *Collector, page wiki.Page) {
	if page.Slug == "_index" {
		return
	}
	if len(strings.TrimSpace(page.Body)) >= 50 {
		return
	}
	c.Add(Diagnostic{
		Level:   Warn,
		File:    page.Path,
		Message: "empty page (<50 char body)",
	})
}

func addPrivateTagDiagnostic(c *Collector, page wiki.Page) {
	if !hasTag(page, "private") || isPrivatePath(page.RelPath) {
		return
	}
	c.Add(Diagnostic{
		Level:   Warn,
		File:    page.Path,
		Message: "private tag outside private path",
	})
}

func addBrokenWikilinkDiagnostics(c *Collector, idx wiki.Index, page wiki.Page) {
	for _, link := range page.Links {
		if _, ok := idx.Resolve(link.Target); ok {
			continue
		}
		c.Add(Diagnostic{
			Level:   Error,
			File:    page.Path,
			Message: fmt.Sprintf("broken wikilink: [[%s]]", link.Target),
		})
	}
}

func addOrphanDiagnostics(c *Collector, idx wiki.Index) {
	for _, page := range idx.Pages {
		if isSystemPage(page) || page.Slug == "_index" || page.Slug == "catalog" {
			continue
		}
		if len(idx.Inbound[page.Slug]) > 0 {
			continue
		}
		c.Add(Diagnostic{
			Level:   Info,
			File:    page.Path,
			Message: "orphan: no inbound wikilinks",
		})
	}
}

func addCatalogDiagnostics(c *Collector, idx wiki.Index) {
	catalog, ok := catalogPage(idx.Pages)
	if !ok {
		return
	}
	catalogLinks := make(map[string]bool)
	for _, link := range catalog.Links {
		catalogLinks[link.Target] = true
	}
	for _, page := range idx.Pages {
		if isSystemPage(page) || page.Slug == "_index" || page.Slug == "catalog" {
			continue
		}
		if catalogLinks[page.Slug] {
			continue
		}
		c.Add(Diagnostic{
			Level:   Warn,
			File:    page.Path,
			Message: "missing from catalog",
		})
	}
}

func hasTag(page wiki.Page, tag string) bool {
	for _, candidate := range page.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func isPrivatePath(relPath string) bool {
	relPath = strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "./")
	return strings.HasPrefix(relPath, "private/") || strings.Contains(relPath, "/private/")
}

func isSystemPage(page wiki.Page) bool {
	switch page.Type {
	case "log", "catalog", "section-index":
		return true
	default:
		return false
	}
}

func catalogPage(pages []wiki.Page) (wiki.Page, bool) {
	for _, page := range pages {
		if page.Slug == "catalog" {
			return page, true
		}
	}
	return wiki.Page{}, false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
