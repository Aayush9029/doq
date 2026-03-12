package symbolgraph

import (
	"encoding/json"
	"fmt"
	"os"
)

// ParseFile reads and decodes a .symbols.json file.
func ParseFile(path string) (*SymbolGraph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var sg SymbolGraph
	dec := json.NewDecoder(f)
	if err := dec.Decode(&sg); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return &sg, nil
}
