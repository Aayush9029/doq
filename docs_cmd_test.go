package main

import (
	"reflect"
	"testing"

	"github.com/Aayush9029/doq/internal/docs"
)

func TestParseDocsSearchArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseDocsSearchArgs([]string{
		"swift", "testing",
		"--framework", "Swift Testing",
		"--kind", "article",
		"--limit", "5",
		"--omit-content",
		"--json",
	})
	if err != nil {
		t.Fatalf("parseDocsSearchArgs() error = %v", err)
	}

	if parsed.query != "swift testing" {
		t.Fatalf("query = %q, want %q", parsed.query, "swift testing")
	}
	if !reflect.DeepEqual(parsed.opts.Frameworks, []string{"Swift Testing"}) {
		t.Fatalf("frameworks = %#v", parsed.opts.Frameworks)
	}
	if !reflect.DeepEqual(parsed.opts.Kinds, []string{docs.KindArticle}) {
		t.Fatalf("kinds = %#v", parsed.opts.Kinds)
	}
	if parsed.opts.Limit != 5 {
		t.Fatalf("limit = %d, want 5", parsed.opts.Limit)
	}
	if !parsed.opts.OmitContent {
		t.Fatalf("omit content = false, want true")
	}
	if !parsed.json {
		t.Fatalf("json = false, want true")
	}
}

func TestParseDocsSearchArgsRejectsUnknownKind(t *testing.T) {
	t.Parallel()

	_, err := parseDocsSearchArgs([]string{"swiftui", "--kind", "guide"})
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestParseDocsGetArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseDocsGetArgs([]string{"/documentation/Testing", "--json"})
	if err != nil {
		t.Fatalf("parseDocsGetArgs() error = %v", err)
	}
	if parsed.identifier != "/documentation/Testing" {
		t.Fatalf("identifier = %q", parsed.identifier)
	}
	if !parsed.json {
		t.Fatalf("json = false, want true")
	}
}
