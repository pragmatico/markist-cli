package cli

import "testing"

func TestDocsOpensTheGuideURL(t *testing.T) {
	openedURL := stubOpenBrowser(t)

	cmd := newDocsCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs RunE error: %v", err)
	}

	if *openedURL != DocsURL {
		t.Fatalf("openBrowser called with %q, want %q", *openedURL, DocsURL)
	}
}
