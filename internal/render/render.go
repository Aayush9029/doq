package render

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Aayush9029/doq/internal/index"
	"github.com/Aayush9029/doq/internal/symbolgraph"
	"github.com/Aayush9029/doq/internal/ui"
)

var (
	// DocC link references: ``View/body-swift.property`` → body
	// Negative lookahead/behind not available in Go regex, so we use a
	// callback to skip matches that are inside code fences (```)
	doccSymRef = regexp.MustCompile("``([^`]+)``")
	// DocC doc links: <doc:Declaring-a-Custom-View> → Declaring a Custom View
	doccDocLink = regexp.MustCompile(`<doc:([^>]+)>`)
)

// KindBadge returns a short badge for the symbol kind.
func KindBadge(kind string) string {
	switch kind {
	case "swift.struct":
		return "[S]"
	case "swift.class":
		return "[C]"
	case "swift.protocol":
		return "[P]"
	case "swift.func", "swift.func.op", "swift.type.method", "swift.method":
		return "[F]"
	case "swift.enum":
		return "[E]"
	case "swift.init":
		return "[I]"
	case "swift.type.alias":
		return "[T]"
	case "swift.var", "swift.type.property", "swift.property":
		return "[V]"
	case "swift.macro":
		return "[#]"
	default:
		return "[·]"
	}
}

// KindColor returns the ANSI color for a kind badge.
func KindColor(kind string) string {
	switch kind {
	case "swift.struct":
		return ui.Cyan
	case "swift.class":
		return ui.Yellow
	case "swift.protocol":
		return ui.Blue
	case "swift.enum":
		return ui.Green
	case "swift.func", "swift.func.op", "swift.type.method", "swift.method":
		return ui.Cyan
	default:
		return ui.Dim
	}
}

// FormatSearchResult formats a single search result as one line.
func FormatSearchResult(r index.SearchResult, color bool) string {
	badge := KindBadge(r.Kind)
	path := r.Path

	if !color {
		return fmt.Sprintf("%-4s %-40s %s  %s", badge, r.Name, r.Module, path)
	}

	kc := KindColor(r.Kind)
	return fmt.Sprintf("%s%s%s %-40s %s%s%s  %s%s%s",
		kc, badge, ui.Reset,
		r.Name,
		ui.Dim, r.Module, ui.Reset,
		ui.Dim, path, ui.Reset,
	)
}

// FormatSymbol formats a full symbol for terminal display.
func FormatSymbol(sym *index.FullSymbol, width int, color bool) string {
	var b strings.Builder

	if width <= 0 {
		width = 80
	}

	// Header: Module > Path                            Kind
	header := sym.Module + " > " + sym.Path
	kindLabel := sym.KindDisplay
	padding := width - len(header) - len(kindLabel)
	if padding < 2 {
		padding = 2
	}

	if color {
		b.WriteString(fmt.Sprintf("%s%s%s%s%s%s%s\n",
			ui.Bold, header, ui.Reset,
			strings.Repeat(" ", padding),
			ui.Dim, kindLabel, ui.Reset,
		))
	} else {
		b.WriteString(fmt.Sprintf("%s%s%s\n", header, strings.Repeat(" ", padding), kindLabel))
	}

	b.WriteString("\n")

	// Declaration
	if sym.Declaration != "" {
		if color {
			b.WriteString(ui.Green)
		}
		b.WriteString(sym.Declaration)
		if color {
			b.WriteString(ui.Reset)
		}
		b.WriteString("\n\n")
	}

	// Doc comment
	if sym.DocComment != "" {
		doc := cleanDocComment(sym.DocComment)
		b.WriteString(wordWrap(doc, width))
		b.WriteString("\n\n")
	}

	// Availability
	if sym.Availability != "" {
		avails := formatAvailability(sym.Availability)
		if avails != "" {
			if color {
				b.WriteString(fmt.Sprintf("%sAvailable:%s %s\n", ui.Bold, ui.Reset, avails))
			} else {
				b.WriteString(fmt.Sprintf("Available: %s\n", avails))
			}
		}
	}

	// Relationships
	if len(sym.ConformsTo) > 0 {
		label := "Conforms to"
		if color {
			b.WriteString(fmt.Sprintf("%s%s:%s %s\n", ui.Bold, label, ui.Reset, strings.Join(sym.ConformsTo, ", ")))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s\n", label, strings.Join(sym.ConformsTo, ", ")))
		}
	}

	if len(sym.InheritsFrom) > 0 {
		label := "Inherits from"
		if color {
			b.WriteString(fmt.Sprintf("%s%s:%s %s\n", ui.Bold, label, ui.Reset, strings.Join(sym.InheritsFrom, ", ")))
		} else {
			b.WriteString(fmt.Sprintf("%s: %s\n", label, strings.Join(sym.InheritsFrom, ", ")))
		}
	}

	if sym.Members > 0 {
		label := "Members"
		if color {
			b.WriteString(fmt.Sprintf("%s%s:%s %d symbols\n", ui.Bold, label, ui.Reset, sym.Members))
		} else {
			b.WriteString(fmt.Sprintf("%s: %d symbols\n", label, sym.Members))
		}
	}

	return b.String()
}

func formatAvailability(availJSON string) string {
	var avails []symbolgraph.Availability
	if err := json.Unmarshal([]byte(availJSON), &avails); err != nil {
		return ""
	}

	var parts []string
	for _, a := range avails {
		if a.Introduced != nil {
			parts = append(parts, fmt.Sprintf("%s %s+", a.Domain, a.Introduced))
		} else if a.Domain != "" {
			parts = append(parts, a.Domain)
		}
	}
	return strings.Join(parts, ", ")
}

// cleanDocComment strips DocC markup from documentation text.
func cleanDocComment(text string) string {
	var out []string
	inCodeBlock := false

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			out = append(out, line)
			continue
		}
		if inCodeBlock {
			out = append(out, line)
			continue
		}

		// ``Symbol/path-swift.property`` → Symbol/path
		line = doccSymRef.ReplaceAllStringFunc(line, func(match string) string {
			inner := match[2 : len(match)-2]
			if i := strings.Index(inner, "-swift."); i >= 0 {
				inner = inner[:i]
			}
			if parts := strings.Split(inner, "/"); len(parts) > 0 {
				return parts[len(parts)-1]
			}
			return inner
		})

		// <doc:Declaring-a-Custom-View> → "Declaring a Custom View"
		line = doccDocLink.ReplaceAllStringFunc(line, func(match string) string {
			inner := match[5 : len(match)-1]
			return strings.ReplaceAll(inner, "-", " ")
		})

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func wordWrap(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if len(paragraph) <= width {
			lines = append(lines, paragraph)
			continue
		}

		words := strings.Fields(paragraph)
		var line string
		for _, w := range words {
			if line == "" {
				line = w
			} else if len(line)+1+len(w) > width {
				lines = append(lines, line)
				line = w
			} else {
				line += " " + w
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
