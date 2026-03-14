package render

import (
	"strings"
	"testing"

	"github.com/Aayush9029/doq/internal/docs"
)

func TestFormatDocSearchBlock(t *testing.T) {
	t.Parallel()

	got := FormatDocSearchBlock(docs.SearchResult{
		ID:        "/documentation/Testing",
		Framework: "Swift Testing",
		Kind:      "article",
		Title:     "Swift Testing",
		Content:   "Overview\n\nLearn to test Swift code.",
		Score:     0.75,
	}, 80, false, 1)

	for _, want := range []string{
		"1. Swift Testing",
		"Framework: Swift Testing",
		"Kind: article",
		"Relevance: 0.7500",
		"ID: /documentation/Testing",
		"Learn to test Swift code.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted block missing %q:\n%s", want, got)
		}
	}
}

func TestFormatDocEntry(t *testing.T) {
	t.Parallel()

	got := FormatDocEntry(&docs.Entry{
		ID:        "/documentation/Testing",
		Framework: "Swift Testing",
		Kind:      "symbol",
		Title:     "Swift Testing",
		Content:   "Overview\n\nTest all the things.",
	}, 80, false)

	for _, want := range []string{
		"Swift Testing",
		"Framework: Swift Testing",
		"Kind: symbol",
		"ID: /documentation/Testing",
		"Test all the things.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted entry missing %q:\n%s", want, got)
		}
	}
}
