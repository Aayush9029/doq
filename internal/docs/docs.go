package docs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	KindArticle = "article"
	KindSymbol  = "symbol"
	KindTopic   = "topic"
)

var (
	ErrUnavailable = errors.New("semantic docs search is unavailable on this build")
	validKinds     = map[string]struct{}{
		KindArticle: {},
		KindSymbol:  {},
		KindTopic:   {},
	}
)

type SearchOptions struct {
	Frameworks  []string
	Kinds       []string
	Limit       int
	OmitContent bool
}

type SearchResult struct {
	ID        string  `json:"id"`
	Framework string  `json:"framework,omitempty"`
	Kind      string  `json:"kind,omitempty"`
	Title     string  `json:"title,omitempty"`
	Content   string  `json:"content,omitempty"`
	Score     float64 `json:"score"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

type Entry struct {
	ID        string `json:"id"`
	Framework string `json:"framework,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content,omitempty"`
}

func Available() error {
	return available()
}

func Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	opts.Frameworks = normalizeList(opts.Frameworks)
	opts.Kinds = normalizeKinds(opts.Kinds)
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	payload, err := searchJSON(query, opts)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, fmt.Errorf("decoding docs search results: %w", err)
	}
	return results, nil
}

func Get(ctx context.Context, identifier string) (*Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("documentation identifier is required")
	}

	payload, err := getJSON(identifier)
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(payload, &entry); err != nil {
		return nil, fmt.Errorf("decoding docs entry: %w", err)
	}
	return &entry, nil
}

func IsValidKind(kind string) bool {
	_, ok := validKinds[strings.ToLower(strings.TrimSpace(kind))]
	return ok
}

func normalizeList(values []string) []string {
	var normalized []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeKinds(values []string) []string {
	var normalized []string
	for _, value := range normalizeList(values) {
		value = strings.ToLower(value)
		if IsValidKind(value) {
			normalized = append(normalized, value)
		}
	}
	return normalized
}
