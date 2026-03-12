package extract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ProgressFunc is called when a module finishes extraction.
type ProgressFunc func(module string, current, total int, err error)

// Extract runs xcrun swift symbolgraph-extract concurrently for the given modules.
// Returns the list of generated .symbols.json file paths.
func Extract(ctx context.Context, sdkPath, target, outputDir string, modules []string, progress ProgressFunc) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}

	type result struct {
		module string
		err    error
	}

	work := make(chan string, len(modules))
	results := make(chan result, len(modules))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mod := range work {
				if ctx.Err() != nil {
					results <- result{mod, ctx.Err()}
					continue
				}
				err := extractModule(ctx, sdkPath, target, outputDir, mod)
				results <- result{mod, err}
			}
		}()
	}

	for _, mod := range modules {
		work <- mod
	}
	close(work)

	go func() {
		wg.Wait()
		close(results)
	}()

	var (
		done    int
		errMods []string
	)
	for r := range results {
		done++
		if r.err != nil {
			errMods = append(errMods, r.module)
		}
		if progress != nil {
			progress(r.module, done, len(modules), r.err)
		}
	}

	// Collect generated files
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("reading output dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".symbols.json") {
			files = append(files, filepath.Join(outputDir, e.Name()))
		}
	}

	if len(files) == 0 && len(errMods) > 0 {
		return nil, fmt.Errorf("all extractions failed, first: %s", errMods[0])
	}

	return files, nil
}

func extractModule(ctx context.Context, sdkPath, target, outputDir, module string) error {
	args := []string{
		"swift", "symbolgraph-extract",
		"-module-name", module,
		"-sdk", sdkPath,
		"-target", target,
		"-output-dir", outputDir,
		"-minimum-access-level", "public",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	cmd.Stderr = nil // suppress warnings
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extracting %s: %w", module, err)
	}
	return nil
}
