package docs

import (
	"context"
	"os"
	"testing"
)

func TestLiveSearchAndGet(t *testing.T) {
	if os.Getenv("DOQ_LIVE_DOCS_TESTS") != "1" {
		t.Skip("set DOQ_LIVE_DOCS_TESTS=1 to run live docs integration tests")
	}

	if err := Available(); err != nil {
		t.Fatalf("Available() error = %v", err)
	}

	ctx := context.Background()
	results, err := Search(ctx, "swift testing", SearchOptions{
		Frameworks: []string{"Swift Testing"},
		Limit:      3,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no results")
	}

	for _, result := range results {
		if result.Framework != "" && result.Framework != "Swift Testing" {
			t.Fatalf("result framework = %q, want Swift Testing", result.Framework)
		}
	}

	entry, err := Get(ctx, "/documentation/Testing")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if entry.ID != "/documentation/Testing" {
		t.Fatalf("entry.ID = %q", entry.ID)
	}
	if entry.Content == "" {
		t.Fatal("entry.Content is empty")
	}
}
