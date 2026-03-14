package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Aayush9029/doq/internal/config"
	"github.com/Aayush9029/doq/internal/extract"
	"github.com/Aayush9029/doq/internal/index"
	"github.com/Aayush9029/doq/internal/render"
	"github.com/Aayush9029/doq/internal/symbolgraph"
	"github.com/Aayush9029/doq/internal/tui"
	"github.com/Aayush9029/doq/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		ui.Fatalf("%s", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		if ui.IsTTY() {
			return runTUI()
		}
		showHelp()
		return nil
	}

	switch args[0] {
	case "search", "s":
		if len(args) < 2 {
			return fmt.Errorf("usage: doq search <term>")
		}
		return cmdSearch(strings.Join(args[1:], " "))

	case "info", "i":
		if len(args) < 2 {
			return fmt.Errorf("usage: doq info <symbol>")
		}
		return cmdInfo(strings.Join(args[1:], " "))

	case "list", "ls":
		return cmdList()

	case "index", "ix":
		return cmdIndex(args[1:])

	case "docs", "d":
		return cmdDocs(args[1:])

	case "--version", "-v", "version":
		fmt.Printf("doq %s\n", version)
		return nil

	case "--help", "-h", "help":
		showHelp()
		return nil

	default:
		// Treat unknown single word as search
		return cmdSearch(strings.Join(args, " "))
	}
}

func showHelp() {
	ui.Header("doq — Apple Developer Documentation")
	fmt.Println("Usage:")
	fmt.Println("  doq                     Launch interactive TUI")
	fmt.Println("  doq search <term>       Search symbols")
	fmt.Println("  doq info <symbol>       Show full symbol details")
	fmt.Println("  doq list                List indexed frameworks")
	fmt.Println("  doq index [frameworks]  Build/rebuild search index")
	fmt.Println("  doq index --all         Index all SDK frameworks")
	fmt.Println("  doq docs                Semantic Apple docs search")
	fmt.Println("  doq --version           Show version")
	fmt.Println()
}

func openIndex() (*index.Index, error) {
	path := config.IndexPath()
	idx, err := index.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}

	ctx := context.Background()
	if !idx.HasSymbols(ctx) {
		idx.Close()
		return nil, fmt.Errorf("index is empty — run 'doq index' first")
	}

	return idx, nil
}

func cmdSearch(query string) error {
	t0 := time.Now()
	idx, err := openIndex()
	if err != nil {
		return err
	}
	defer idx.Close()
	tOpen := time.Since(t0)

	ctx := context.Background()
	t1 := time.Now()
	results, err := idx.Search(ctx, query, 20)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	tSearch := time.Since(t1)

	if len(results) == 0 {
		ui.Dimf("No results for %q", query)
		return nil
	}

	color := ui.IsTTY()
	for _, r := range results {
		fmt.Println(render.FormatSearchResult(r, color))
	}

	if os.Getenv("DOQ_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "\n%sopen=%v search=%v total=%v%s\n", ui.Dim, tOpen, tSearch, time.Since(t0), ui.Reset)
	}
	return nil
}

func cmdInfo(symbol string) error {
	idx, err := openIndex()
	if err != nil {
		return err
	}
	defer idx.Close()

	ctx := context.Background()

	// Try as precise ID first, then as name
	var sym *index.FullSymbol
	sym, err = idx.GetSymbol(ctx, symbol)
	if err != nil {
		sym, err = idx.GetSymbolByName(ctx, symbol)
		if err != nil {
			return err
		}
	}

	width := ui.TermWidth()
	color := ui.IsTTY()
	fmt.Println(render.FormatSymbol(sym, width, color))
	return nil
}

func cmdList() error {
	idx, err := openIndex()
	if err != nil {
		return err
	}
	defer idx.Close()

	ctx := context.Background()
	modules, err := idx.ListModules(ctx)
	if err != nil {
		return fmt.Errorf("listing modules: %w", err)
	}

	if len(modules) == 0 {
		ui.Dimf("No frameworks indexed — run 'doq index' first")
		return nil
	}

	color := ui.IsTTY()
	if color {
		ui.Header("Indexed Frameworks")
	}

	var total int
	for _, m := range modules {
		total += m.SymbolCount
		if color {
			fmt.Printf("  %s%-25s%s %s%d symbols%s\n", ui.Bold, m.Name, ui.Reset, ui.Dim, m.SymbolCount, ui.Reset)
		} else {
			fmt.Printf("%-25s %d symbols\n", m.Name, m.SymbolCount)
		}
	}

	if color {
		fmt.Printf("\n  %s%d frameworks, %d total symbols%s\n", ui.Dim, len(modules), total, ui.Reset)
	}
	return nil
}

func cmdIndex(args []string) error {
	start := time.Now()
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	sdkPath := cfg.SDKPath
	if sdkPath == "" {
		sp := ui.NewSpinner("Detecting SDK...")
		sdkPath, err = config.DetectSDK()
		sp.Stop()
		if err != nil {
			return err
		}
	}

	target := cfg.Target
	if target == "" {
		target = config.DetectTarget()
	}

	// Determine frameworks to index
	var frameworks []string
	useAll := false
	for _, a := range args {
		if a == "--all" {
			useAll = true
			break
		}
	}

	if useAll {
		frameworks, err = config.ListAllFrameworks(sdkPath)
		if err != nil {
			return err
		}
		ui.Status(fmt.Sprintf("Indexing all %d frameworks from SDK", len(frameworks)))
	} else if len(args) > 0 {
		frameworks = args
	} else {
		frameworks = cfg.Frameworks
	}

	ui.Header("doq — Building Documentation Index")
	ui.Status(fmt.Sprintf("SDK: %s", sdkPath))
	ui.Status(fmt.Sprintf("Target: %s", target))
	ui.Status(fmt.Sprintf("Frameworks: %d", len(frameworks)))
	fmt.Println()

	// Extract symbol graphs
	tmpDir, err := os.MkdirTemp("", "doq-extract-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	var failed int

	progress := func(module string, current, total int, err error) {
		if err != nil {
			failed++
		}
		ui.ProgressBar(fmt.Sprintf("Extracting · %s", module), current, total)
	}

	files, err := extract.Extract(ctx, sdkPath, target, tmpDir, frameworks, progress)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	msg := fmt.Sprintf("Extracted %d symbol graphs", len(files))
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	ui.ProgressDone(msg)

	// Parse symbol graphs
	var graphs []*symbolgraph.SymbolGraph
	for i, f := range files {
		ui.ProgressBar("Parsing", i+1, len(files))
		sg, err := symbolgraph.ParseFile(f)
		if err != nil {
			continue
		}
		graphs = append(graphs, sg)
	}
	ui.ProgressDone(fmt.Sprintf("Parsed %d symbol graphs", len(graphs)))

	// Build index
	sp := ui.NewSpinner("Building search index...")
	idx, err := index.Open(config.IndexPath())
	if err != nil {
		sp.Stop()
		return err
	}
	defer idx.Close()

	if err := idx.Build(ctx, graphs); err != nil {
		sp.Stop()
		return fmt.Errorf("building index: %w", err)
	}

	idx.SetMeta("sdk_path", sdkPath)
	idx.SetMeta("doq_version", version)

	count := idx.SymbolCount(ctx)
	sp.Stop()
	ui.Success(fmt.Sprintf("Indexed %d symbols", count))

	sp = ui.NewSpinner("Compacting database...")
	idx.Compact()
	sp.Stop()
	ui.Success(fmt.Sprintf("Saved -> %s", config.IndexPath()))
	fmt.Println()
	ui.Dimf("Done in %s", time.Since(start).Round(time.Millisecond))
	return nil
}

func runTUI() error {
	idx, err := openIndex()
	if err != nil {
		return err
	}

	m := tui.NewModel(idx)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
