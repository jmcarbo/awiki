package dataset

import (
	"fmt"
	"strings"
)

// ScaffoldInput holds the parameters for generating a new dataset page.
type ScaffoldInput struct {
	Slug   string
	Format Format
	Today  string
	Rows   int
	Body   []byte // optional inline data body (fence content)
}

// WriteScaffold produces the byte content for a new dataset page, matching
// byte-for-byte the cmd_new heredoc in scripts/dataset.sh.
func WriteScaffold(in ScaffoldInput) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: \"%s\"\n", in.Slug)
	fmt.Fprintf(&b, "date: %s\n", in.Today)
	fmt.Fprintf(&b, "last_updated: %s\n", in.Today)
	b.WriteString("type: dataset\n")
	b.WriteString("tags: []\n")
	b.WriteString("storage: inline\n")
	fmt.Fprintf(&b, "format: %s\n", string(in.Format))
	fmt.Fprintf(&b, "rows: %d\n", in.Rows)
	b.WriteString("sources: []\n")
	b.WriteString("draft: false\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", in.Slug)
	b.WriteString("<!-- one-paragraph description here -->\n\n")
	b.WriteString("## Schema\n\n")
	b.WriteString("<!-- describe each column here -->\n\n")
	b.WriteString("## Data\n")
	fmt.Fprintf(&b, "```%s\n", string(in.Format))
	if len(in.Body) > 0 {
		body := string(in.Body)
		// Ensure the body ends with a newline before the closing fence.
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		b.WriteString(body)
	}
	b.WriteString("```\n\n")
	b.WriteString("## Provenance\n\n")
	b.WriteString("<!-- where these rows came from -->\n\n")
	b.WriteString("## Related\n\n")
	b.WriteString("## Sources\n")

	return []byte(b.String())
}
