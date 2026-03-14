package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Aayush9029/doq/internal/docs"
	"github.com/Aayush9029/doq/internal/render"
	"github.com/Aayush9029/doq/internal/tui"
	"github.com/Aayush9029/doq/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type docsSearchArgs struct {
	query string
	opts  docs.SearchOptions
	json  bool
}

type docsGetArgs struct {
	identifier string
	json       bool
}

func cmdDocs(args []string) error {
	if len(args) == 0 {
		if ui.IsTTY() {
			return runDocsTUI()
		}
		showDocsHelp()
		return nil
	}

	switch args[0] {
	case "search", "s":
		return cmdDocsSearch(args[1:])
	case "get", "g":
		return cmdDocsGet(args[1:])
	case "--help", "-h", "help":
		showDocsHelp()
		return nil
	default:
		return cmdDocsSearch(args)
	}
}

func cmdDocsSearch(args []string) error {
	parsed, err := parseDocsSearchArgs(args)
	if err != nil {
		return err
	}

	results, err := docs.Search(context.Background(), parsed.query, parsed.opts)
	if err != nil {
		return err
	}

	if parsed.json {
		return printJSON(docs.SearchResponse{Results: results})
	}

	if len(results) == 0 {
		ui.Dimf("No semantic docs results for %q", parsed.query)
		return nil
	}

	width := ui.TermWidth()
	color := ui.IsTTY()
	for i, result := range results {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(render.FormatDocSearchBlock(result, width, color, i+1))
	}
	return nil
}

func cmdDocsGet(args []string) error {
	parsed, err := parseDocsGetArgs(args)
	if err != nil {
		return err
	}

	entry, err := docs.Get(context.Background(), parsed.identifier)
	if err != nil {
		return err
	}

	if parsed.json {
		return printJSON(entry)
	}

	fmt.Println(render.FormatDocEntry(entry, ui.TermWidth(), ui.IsTTY()))
	return nil
}

func showDocsHelp() {
	ui.Header("doq docs — Semantic Apple Documentation Search")
	fmt.Println("Usage:")
	fmt.Println("  doq docs                           Launch semantic docs TUI")
	fmt.Println("  doq docs search <query>            Search Apple docs semantically")
	fmt.Println("  doq docs get <identifier>          Fetch a documentation entry")
	fmt.Println("  doq docs search --framework SwiftUI --kind article \"list row\"")
	fmt.Println()
}

func parseDocsSearchArgs(args []string) (docsSearchArgs, error) {
	parsed := docsSearchArgs{
		opts: docs.SearchOptions{Limit: 10},
	}

	var queryParts []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--framework":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("usage: doq docs search <query> [--framework <name>] [--kind <kind>] [--limit <n>] [--omit-content] [--json]")
			}
			i++
			parsed.opts.Frameworks = append(parsed.opts.Frameworks, args[i])
		case "--kind":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("usage: doq docs search <query> [--framework <name>] [--kind <kind>] [--limit <n>] [--omit-content] [--json]")
			}
			i++
			kind := strings.ToLower(strings.TrimSpace(args[i]))
			if !docs.IsValidKind(kind) {
				return parsed, fmt.Errorf("unsupported docs kind %q (expected article, symbol, or topic)", args[i])
			}
			parsed.opts.Kinds = append(parsed.opts.Kinds, kind)
		case "--limit":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("usage: doq docs search <query> [--framework <name>] [--kind <kind>] [--limit <n>] [--omit-content] [--json]")
			}
			i++
			limit, err := strconv.Atoi(args[i])
			if err != nil || limit <= 0 {
				return parsed, fmt.Errorf("invalid --limit value %q", args[i])
			}
			parsed.opts.Limit = limit
		case "--omit-content":
			parsed.opts.OmitContent = true
		case "--json":
			parsed.json = true
		default:
			if strings.HasPrefix(arg, "--") {
				return parsed, fmt.Errorf("unknown docs search flag %q", arg)
			}
			queryParts = append(queryParts, arg)
		}
	}

	parsed.query = strings.TrimSpace(strings.Join(queryParts, " "))
	if parsed.query == "" {
		return parsed, fmt.Errorf("usage: doq docs search <query> [--framework <name>] [--kind <kind>] [--limit <n>] [--omit-content] [--json]")
	}
	return parsed, nil
}

func parseDocsGetArgs(args []string) (docsGetArgs, error) {
	var parsed docsGetArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			parsed.json = true
		default:
			if strings.HasPrefix(arg, "--") {
				return parsed, fmt.Errorf("unknown docs get flag %q", arg)
			}
			if parsed.identifier != "" {
				return parsed, fmt.Errorf("usage: doq docs get <identifier> [--json]")
			}
			parsed.identifier = arg
		}
	}

	if strings.TrimSpace(parsed.identifier) == "" {
		return parsed, fmt.Errorf("usage: doq docs get <identifier> [--json]")
	}
	return parsed, nil
}

func runDocsTUI() error {
	if err := docs.Available(); err != nil {
		return err
	}

	m := tui.NewDocsModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
