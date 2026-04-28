package wiki

import (
	"reflect"
	"testing"
)

func TestBuildIndexResolvesDirectSlugsAndAliases(t *testing.T) {
	pages := []Page{
		pageForIndex("alpha", "Alpha"),
		pageForIndex("beta", "Beta"),
	}

	idx := BuildIndex(pages)

	got, ok := idx.Resolve("alpha")
	if !ok || got.Slug != "alpha" {
		t.Fatalf("Resolve(alpha) = %#v, %v; want alpha, true", got, ok)
	}
	got, ok = idx.Resolve("Beta")
	if !ok || got.Slug != "beta" {
		t.Fatalf("Resolve(Beta) = %#v, %v; want beta, true", got, ok)
	}
	if got, want := idx.Aliases["Beta"], "beta"; got != want {
		t.Fatalf("Aliases[Beta] = %q, want %q", got, want)
	}
}

func TestBuildIndexRecordsInboundLinksToResolvedTargets(t *testing.T) {
	source := pageForIndex("source")
	source.Links = []Link{
		{Target: "target"},
		{Target: "Target Alias"},
		{Target: "missing"},
	}
	target := pageForIndex("target", "Target Alias")

	idx := BuildIndex([]Page{source, target})

	assertPageSlugs(t, "Inbound[target]", idx.Inbound["target"], []string{"source", "source"})
	if _, ok := idx.Inbound["missing"]; ok {
		t.Fatalf("Inbound[missing] exists for unresolved link")
	}
}

func TestBuildIndexRecordsInboundLinksFromFrontmatterSources(t *testing.T) {
	referrer := pageForIndex("referrer")
	referrer.Sources = []string{"[[source]]"}
	source := pageForIndex("source")

	idx := BuildIndex([]Page{referrer, source})

	assertPageSlugs(t, "Inbound[source]", idx.Inbound["source"], []string{"referrer"})
}

func TestBuildIndexDuplicateSlugDetectionIgnoresIndexPages(t *testing.T) {
	first := pageForIndex("duplicate")
	second := pageForIndex("duplicate")
	indexOne := pageForIndex("_index")
	indexTwo := pageForIndex("_index")

	idx := BuildIndex([]Page{first, second, indexOne, indexTwo})

	if got, want := idx.BySlug["duplicate"], first; !reflect.DeepEqual(got, want) {
		t.Fatalf("BySlug[duplicate] = %#v, want first page %#v", got, want)
	}
	assertPageSlugs(t, "DuplicateSlugs[duplicate]", idx.DuplicateSlugs["duplicate"], []string{"duplicate", "duplicate"})
	if _, ok := idx.DuplicateSlugs["_index"]; ok {
		t.Fatalf("DuplicateSlugs[_index] exists, want ignored")
	}
}

func TestBuildIndexAliasCollisionDetectionRecordsAliasesUsedByMultiplePages(t *testing.T) {
	alpha := pageForIndex("alpha", "Shared")
	beta := pageForIndex("beta", "Shared")

	idx := BuildIndex([]Page{alpha, beta})

	assertPageSlugs(t, "AliasCollision[Shared]", idx.AliasCollision["Shared"], []string{"alpha", "beta"})
	if _, ok := idx.Aliases["Shared"]; ok {
		t.Fatalf("Aliases[Shared] exists for colliding alias")
	}
	if got, ok := idx.Resolve("Shared"); ok {
		t.Fatalf("Resolve(Shared) = %#v, true; want unresolved collision", got)
	}
}

func TestBuildIndexCapturesCatalogBody(t *testing.T) {
	catalog := pageForIndex("catalog")
	catalog.Body = "Catalog contents.\n"

	idx := BuildIndex([]Page{pageForIndex("alpha"), catalog})

	if idx.CatalogBody != "Catalog contents.\n" {
		t.Fatalf("CatalogBody = %q, want catalog body", idx.CatalogBody)
	}
}

func TestBuildIndexCapturesOnlyRootCatalogBody(t *testing.T) {
	privateCatalog := pageForIndex("catalog")
	privateCatalog.RelPath = "private/catalog.md"
	privateCatalog.Body = "Private catalog contents.\n"

	idx := BuildIndex([]Page{pageForIndex("alpha"), privateCatalog})

	if idx.CatalogBody != "" {
		t.Fatalf("CatalogBody = %q, want empty without root catalog.md", idx.CatalogBody)
	}
}

func pageForIndex(slug string, aliases ...string) Page {
	return Page{
		Path:    slug + ".md",
		RelPath: slug + ".md",
		Slug:    slug,
		Title:   slug,
		Aliases: aliases,
	}
}

func assertPageSlugs(t *testing.T, name string, pages []Page, want []string) {
	t.Helper()
	var got []string
	for _, page := range pages {
		got = append(got, page.Slug)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s slugs = %#v, want %#v", name, got, want)
	}
}
