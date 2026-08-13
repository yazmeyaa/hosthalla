package layout

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestGenericLayoutIncludesMetadata(t *testing.T) {
	var output bytes.Buffer
	if err := GenericLayout(GenericLayoutProps{
		Title:       "Test",
		Description: "Test description.",
	}).Render(context.Background(), &output); err != nil {
		t.Fatalf("render layout: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`<meta name="description" content="Test description.">`,
		`<meta property="og:title" content="Hosthalla - Test">`,
		`<meta property="og:description" content="Test description.">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:site_name" content="Hosthalla">`,
		`<meta property="og:image" content="/assets/favicon.svg">`,
		`<meta property="og:image:type" content="image/svg+xml">`,
		`<meta property="og:image:alt" content="Hosthalla HH logo">`,
		`<link rel="icon" href="/assets/favicon.svg" type="image/svg+xml" sizes="any">`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("layout is missing %q", expected)
		}
	}
}
