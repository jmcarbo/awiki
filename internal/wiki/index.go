package wiki

type Index struct {
	Pages          []Page
	BySlug         map[string]Page
	Aliases        map[string]string
	AliasCollision map[string][]Page
	DuplicateSlugs map[string][]Page
	Inbound        map[string][]Page
	CatalogBody    string
}

func BuildIndex(pages []Page) Index {
	idx := Index{
		Pages:          append([]Page(nil), pages...),
		BySlug:         make(map[string]Page),
		Aliases:        make(map[string]string),
		AliasCollision: make(map[string][]Page),
		DuplicateSlugs: make(map[string][]Page),
		Inbound:        make(map[string][]Page),
	}

	seenSlugs := make(map[string][]Page)
	aliasOwners := make(map[string][]Page)
	for _, page := range pages {
		if page.Slug == "catalog" {
			idx.CatalogBody = page.Body
		}

		seenSlugs[page.Slug] = append(seenSlugs[page.Slug], page)
		if _, ok := idx.BySlug[page.Slug]; !ok {
			idx.BySlug[page.Slug] = page
		}

		for _, alias := range page.Aliases {
			aliasOwners[alias] = append(aliasOwners[alias], page)
		}
	}

	for slug, owners := range seenSlugs {
		if slug != "_index" && len(owners) > 1 {
			idx.DuplicateSlugs[slug] = owners
		}
	}

	for alias, owners := range aliasOwners {
		if len(owners) == 1 {
			idx.Aliases[alias] = owners[0].Slug
			continue
		}
		idx.AliasCollision[alias] = owners
	}

	for _, page := range pages {
		for _, link := range page.Links {
			target, ok := idx.Resolve(link.Target)
			if !ok {
				continue
			}
			idx.Inbound[target.Slug] = append(idx.Inbound[target.Slug], page)
		}
	}

	return idx
}

func (idx Index) Resolve(target string) (Page, bool) {
	if page, ok := idx.BySlug[target]; ok {
		return page, true
	}
	slug, ok := idx.Aliases[target]
	if !ok {
		return Page{}, false
	}
	page, ok := idx.BySlug[slug]
	return page, ok
}
