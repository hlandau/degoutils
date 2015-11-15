package tpl

import "github.com/flosch/pongo2"
import "github.com/russross/blackfriday"
import "github.com/microcosm-cc/bluemonday"
import "github.com/jackc/pgx"

type Formatted struct {
	StoredValue string
}

func (f *Formatted) SafeHTML() *pongo2.Value {
	return formatTextP2(f.StoredValue)
}

func (f *Formatted) Scan(r *pgx.ValueReader) error {
	if r.Len() == -1 {
		return nil
	}

	f.StoredValue = r.ReadString(r.Len())
	return r.Err()
}

// Takes text to be formatted as input and returns HTML.
//
// The HTML is safe.
func formatText(s string) string {
	return string(bluemonday.UGCPolicy().SanitizeBytes(blackfriday.MarkdownCommon([]byte(s))))
}

func formatTextP2(s string) *pongo2.Value {
	return pongo2.AsSafeValue(formatText(s))
}
