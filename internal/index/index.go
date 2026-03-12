package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Aayush9029/doq/internal/symbolgraph"
	_ "modernc.org/sqlite"
)

const ftsCreate = `CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, doc_comment, content=symbols, content_rowid=id)`

// Index wraps the SQLite documentation index.
type Index struct {
	db *sql.DB
}

// Open opens or creates the index at the given path.
func Open(path string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	idx := &Index{db: db}

	// Set PRAGMAs (always needed)
	db.Exec(`PRAGMA busy_timeout = 5000`)
	db.Exec(`PRAGMA journal_mode = WAL`)

	// Only run full schema init if tables don't exist yet
	if !idx.tableExists("symbols") {
		if err := idx.initSchema(); err != nil {
			db.Close()
			return nil, err
		}
	}

	return idx, nil
}

func (idx *Index) tableExists(name string) bool {
	var n string
	err := idx.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return err == nil
}

func (idx *Index) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			kind_display TEXT NOT NULL,
			module TEXT NOT NULL,
			precise_id TEXT UNIQUE,
			path TEXT NOT NULL,
			declaration TEXT,
			doc_comment TEXT,
			parent_id TEXT,
			availability TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name COLLATE NOCASE)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_module ON symbols(module)`,
		`CREATE INDEX IF NOT EXISTS idx_symbols_parent ON symbols(parent_id)`,
		`CREATE TABLE IF NOT EXISTS relationships (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			target_fallback TEXT,
			PRIMARY KEY (source_id, target_id, kind)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_source ON relationships(source_id)`,
		ftsCreate,
	}
	for _, stmt := range stmts {
		if _, err := idx.db.Exec(stmt); err != nil {
			return fmt.Errorf("schema init: %w", err)
		}
	}
	return nil
}

// Close closes the database.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// SetMeta stores a key-value pair in the meta table.
func (idx *Index) SetMeta(key, value string) error {
	_, err := idx.db.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// GetMeta retrieves a value from the meta table.
func (idx *Index) GetMeta(key string) (string, error) {
	var val string
	err := idx.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// Build inserts symbols and relationships from parsed symbol graphs.
func (idx *Index) Build(ctx context.Context, graphs []*symbolgraph.SymbolGraph) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing data
	tx.Exec(`DELETE FROM symbols`)
	tx.Exec(`DELETE FROM relationships`)
	tx.Exec(`DELETE FROM symbols_fts`)

	symStmt, err := tx.Prepare(`INSERT OR IGNORE INTO symbols(name, kind, kind_display, module, precise_id, path, declaration, doc_comment, parent_id, availability) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing symbol insert: %w", err)
	}
	defer symStmt.Close()

	relStmt, err := tx.Prepare(`INSERT OR IGNORE INTO relationships(source_id, target_id, kind, target_fallback) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing relationship insert: %w", err)
	}
	defer relStmt.Close()

	// Build a map of memberOf relationships to find parent IDs
	memberOf := make(map[string]string) // source precise_id → target precise_id
	for _, g := range graphs {
		for _, r := range g.Relationships {
			if r.Kind == "memberOf" {
				memberOf[r.Source] = r.Target
			}
		}
	}

	for _, g := range graphs {
		module := g.Module.Name

		for _, sym := range g.Symbols {
			if sym.SPI || sym.AccessLevel == "private" || sym.AccessLevel == "internal" {
				continue
			}

			path := strings.Join(sym.PathComponents, "/")
			decl := symbolgraph.DeclarationString(sym.DeclarationFragments)
			doc := symbolgraph.DocCommentText(sym.DocComment)
			parentID := memberOf[sym.Identifier.Precise]

			var availJSON string
			if len(sym.Availability) > 0 {
				b, _ := json.Marshal(sym.Availability)
				availJSON = string(b)
			}

			_, err := symStmt.Exec(
				sym.Names.Title,
				sym.Kind.Identifier,
				sym.Kind.DisplayName,
				module,
				sym.Identifier.Precise,
				path,
				decl,
				doc,
				nullStr(parentID),
				nullStr(availJSON),
			)
			if err != nil {
				return fmt.Errorf("inserting symbol %s: %w", sym.Names.Title, err)
			}
		}

		for _, r := range g.Relationships {
			relStmt.Exec(r.Source, r.Target, r.Kind, nullStr(r.TargetFallback))
		}
	}

	// Rebuild FTS index
	if _, err := tx.Exec(`INSERT INTO symbols_fts(symbols_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuilding FTS: %w", err)
	}

	// Set metadata
	now := time.Now().Format(time.RFC3339)
	tx.Exec(`INSERT INTO meta(key, value) VALUES('indexed_at', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, now)

	return tx.Commit()
}

// SearchResult holds a search hit.
type SearchResult struct {
	ID          int64
	Name        string
	Kind        string
	KindDisplay string
	Module      string
	Path        string
	Declaration string
	DocSnippet  string
}

// Search performs a symbol search. Uses fast name LIKE first, falls back to
// FTS5 for doc-comment full-text search only if name matching finds nothing.
func (idx *Index) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// Fast path: name-based LIKE search (uses idx_symbols_name index)
	results, err := idx.searchByName(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return results, nil
	}

	// Slow path: FTS5 full-text search across name + doc_comment
	return idx.searchFTS(ctx, query, limit)
}

func (idx *Index) searchByName(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	rows, err := idx.db.QueryContext(ctx, `
		SELECT id, name, kind, kind_display, module, path, declaration,
		       COALESCE(substr(doc_comment, 1, 120), '')
		FROM symbols
		WHERE name LIKE ? COLLATE NOCASE
		ORDER BY
			CASE WHEN name = ? COLLATE NOCASE THEN 0
			     WHEN name LIKE ? COLLATE NOCASE THEN 1
			     ELSE 2 END,
			CASE kind
				WHEN 'swift.struct' THEN 0
				WHEN 'swift.class' THEN 0
				WHEN 'swift.protocol' THEN 0
				WHEN 'swift.enum' THEN 0
				ELSE 1
			END,
			length(name)
		LIMIT ?
	`, "%"+query+"%", query, query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.KindDisplay, &r.Module, &r.Path, &r.Declaration, &r.DocSnippet); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (idx *Index) searchFTS(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	terms := strings.Fields(query)
	var validTerms []string
	for _, t := range terms {
		// Strip characters that break FTS5 syntax
		t = strings.ReplaceAll(t, `"`, "")
		t = strings.ReplaceAll(t, `'`, "")
		t = strings.ReplaceAll(t, `*`, "")
		t = strings.TrimSpace(t)
		if t != "" {
			validTerms = append(validTerms, `"`+t+`"*`)
		}
	}
	if len(validTerms) == 0 {
		return nil, nil
	}
	ftsQuery := strings.Join(validTerms, " ")

	rows, err := idx.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.kind_display, s.module, s.path, s.declaration,
		       COALESCE(substr(s.doc_comment, 1, 120), '')
		FROM symbols_fts f
		JOIN symbols s ON s.id = f.rowid
		WHERE symbols_fts MATCH ?
		ORDER BY bm25(symbols_fts, 10.0, 1.0)
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		// FTS syntax error from malformed input — not a real error
		return nil, nil
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.KindDisplay, &r.Module, &r.Path, &r.Declaration, &r.DocSnippet); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// FullSymbol is a symbol with all its details and relationships.
type FullSymbol struct {
	SearchResult
	PreciseID    string
	DocComment   string
	ParentID     *string
	Availability string
	ConformsTo   []string
	InheritsFrom []string
	Members      int
}

// GetSymbol retrieves a full symbol by precise ID.
func (idx *Index) GetSymbol(ctx context.Context, preciseID string) (*FullSymbol, error) {
	var s FullSymbol
	var parentID, availability sql.NullString
	err := idx.db.QueryRowContext(ctx, `
		SELECT id, name, kind, kind_display, module, precise_id, path, declaration,
		       COALESCE(doc_comment, ''), parent_id, availability
		FROM symbols WHERE precise_id = ?
	`, preciseID).Scan(
		&s.ID, &s.Name, &s.Kind, &s.KindDisplay, &s.Module, &s.PreciseID,
		&s.Path, &s.Declaration, &s.DocComment, &parentID, &availability,
	)
	if err != nil {
		return nil, fmt.Errorf("symbol not found: %w", err)
	}

	if parentID.Valid {
		s.ParentID = &parentID.String
	}
	if availability.Valid {
		s.Availability = availability.String
	}

	// Get conformsTo relationships — resolve target to human-readable name
	rows, _ := idx.db.QueryContext(ctx, `
		SELECT COALESCE(s.name, NULLIF(r.target_fallback, ''), r.target_id)
		FROM relationships r
		LEFT JOIN symbols s ON s.precise_id = r.target_id
		WHERE r.source_id = ? AND r.kind = 'conformsTo'
	`, preciseID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)
			if n := cleanRelName(name); n != "" {
				s.ConformsTo = append(s.ConformsTo, n)
			}
		}
	}

	// Get inheritsFrom relationships
	rows2, _ := idx.db.QueryContext(ctx, `
		SELECT COALESCE(s.name, NULLIF(r.target_fallback, ''), r.target_id)
		FROM relationships r
		LEFT JOIN symbols s ON s.precise_id = r.target_id
		WHERE r.source_id = ? AND r.kind = 'inheritsFrom'
	`, preciseID)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var name string
			rows2.Scan(&name)
			if n := cleanRelName(name); n != "" {
				s.InheritsFrom = append(s.InheritsFrom, n)
			}
		}
	}

	// Count members
	idx.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM symbols WHERE parent_id = ?
	`, preciseID).Scan(&s.Members)

	return &s, nil
}

// GetSymbolByName retrieves the best matching symbol by name.
func (idx *Index) GetSymbolByName(ctx context.Context, name string) (*FullSymbol, error) {
	var preciseID string
	err := idx.db.QueryRowContext(ctx, `
		SELECT precise_id FROM symbols
		WHERE name = ? COLLATE NOCASE
		ORDER BY
			CASE kind
				WHEN 'swift.struct' THEN 0
				WHEN 'swift.class' THEN 0
				WHEN 'swift.protocol' THEN 0
				WHEN 'swift.enum' THEN 0
				WHEN 'swift.type.alias' THEN 1
				ELSE 2
			END,
			length(path)
		LIMIT 1
	`, name).Scan(&preciseID)
	if err != nil {
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	return idx.GetSymbol(ctx, preciseID)
}

// ListModules returns all indexed modules.
func (idx *Index) ListModules(ctx context.Context) ([]ModuleInfo, error) {
	rows, err := idx.db.QueryContext(ctx, `
		SELECT module, COUNT(*) as count
		FROM symbols
		GROUP BY module
		ORDER BY module
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []ModuleInfo
	for rows.Next() {
		var m ModuleInfo
		if err := rows.Scan(&m.Name, &m.SymbolCount); err != nil {
			continue
		}
		modules = append(modules, m)
	}
	return modules, rows.Err()
}

// ModuleInfo holds module name and symbol count.
type ModuleInfo struct {
	Name        string
	SymbolCount int
}

// HasSymbols returns true if there are any indexed symbols (fast, no COUNT).
func (idx *Index) HasSymbols(ctx context.Context) bool {
	var id int64
	err := idx.db.QueryRowContext(ctx, `SELECT id FROM symbols LIMIT 1`).Scan(&id)
	return err == nil
}

// SymbolCount returns the total number of indexed symbols.
func (idx *Index) SymbolCount(ctx context.Context) int {
	var count int
	idx.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM symbols`).Scan(&count)
	return count
}

// Compact runs WAL checkpoint and VACUUM to reduce file size.
func (idx *Index) Compact() error {
	idx.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	_, err := idx.db.Exec(`VACUUM`)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// cleanRelName strips module prefixes (e.g. "Swift.Sendable" → "Sendable")
// and handles raw precise IDs that couldn't be resolved.
func cleanRelName(name string) string {
	// If it looks like a precise ID (starts with "s:" or "c:"), skip it
	if strings.HasPrefix(name, "s:") || strings.HasPrefix(name, "c:") {
		return ""
	}
	// Strip module prefix: "Swift.Sendable" → "Sendable"
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}
