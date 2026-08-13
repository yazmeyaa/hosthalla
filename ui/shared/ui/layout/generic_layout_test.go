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
		Title:         "Hosthalla – Test Infrastructure Dashboard Page",
		Description:   "Test description for the Hosthalla infrastructure dashboard with enough context to validate standard and social metadata output.",
		WebOrigin:     "https://hosthalla.example.com",
		CanonicalPath: "/test",
	}).Render(context.Background(), &output); err != nil {
		t.Fatalf("render layout: %v", err)
	}

	html := output.String()
	for _, expected := range []string{
		`<html lang="en" prefix="og: https://ogp.me/ns#">`,
		`<title>Hosthalla – Test Infrastructure Dashboard Page</title>`,
		`<meta name="description" content="Test description for the Hosthalla infrastructure dashboard with enough context to validate standard and social metadata output.">`,
		`<link rel="canonical" href="https://hosthalla.example.com/test">`,
		`<meta property="og:title" content="Hosthalla – Test Infrastructure Dashboard Page">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:site_name" content="Hosthalla">`,
		`<meta property="og:url" content="https://hosthalla.example.com/test">`,
		`<meta property="og:image" content="https://hosthalla.example.com/assets/static/og-preview.png">`,
		`<meta property="og:image:type" content="image/png">`,
		`<meta property="og:image:width" content="1200">`,
		`<meta property="og:image:height" content="630">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta name="twitter:image" content="https://hosthalla.example.com/assets/static/og-preview.png">`,
		`<link rel="icon" href="/assets/favicon.svg" type="image/svg+xml" sizes="any">`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("layout is missing %q", expected)
		}
	}
}
