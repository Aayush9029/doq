package render

import (
	"fmt"
	"strings"

	"github.com/Aayush9029/doq/internal/docs"
	"github.com/Aayush9029/doq/internal/ui"
)

func FormatDocSearchLine(r docs.SearchResult, color bool) string {
	title := r.Title
	if title == "" {
		title = r.ID
	}

	meta := strings.TrimSpace(strings.Join([]string{
		emptyFallback(r.Framework, "Docs"),
		emptyFallback(r.Kind, "entry"),
		fmt.Sprintf("%.4f", r.Score),
	}, " · "))

	if !color {
		return fmt.Sprintf("%-44s %s", truncateRunes(title, 44), meta)
	}

	return fmt.Sprintf("%s%-44s%s %s%s%s",
		ui.Bold, truncateRunes(title, 44), ui.Reset,
		ui.Dim, meta, ui.Reset,
	)
}

func FormatDocSearchBlock(r docs.SearchResult, width int, color bool, index int) string {
	var b strings.Builder

	title := r.Title
	if title == "" {
		title = r.ID
	}
	prefix := ""
	if index > 0 {
		prefix = fmt.Sprintf("%d. ", index)
	}

	if color {
		b.WriteString(ui.Bold + prefix + title + ui.Reset + "\n")
	} else {
		b.WriteString(prefix + title + "\n")
	}

	writeDocMeta(&b, "Framework", r.Framework, color)
	writeDocMeta(&b, "Kind", r.Kind, color)
	writeDocMeta(&b, "Relevance", fmt.Sprintf("%.4f", r.Score), color)
	writeDocMeta(&b, "ID", r.ID, color)

	excerpt := searchExcerpt(r.Content)
	if excerpt != "" {
		b.WriteString("\n")
		b.WriteString(wordWrap(excerpt, width))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func FormatDocEntry(entry *docs.Entry, width int, color bool) string {
	var b strings.Builder

	title := entry.Title
	if title == "" {
		title = entry.ID
	}
	if color {
		b.WriteString(ui.Bold + title + ui.Reset + "\n\n")
	} else {
		b.WriteString(title + "\n\n")
	}

	writeDocMeta(&b, "Framework", entry.Framework, color)
	writeDocMeta(&b, "Kind", entry.Kind, color)
	writeDocMeta(&b, "ID", entry.ID, color)

	if entry.Content != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(wordWrap(cleanDocComment(entry.Content), width))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func writeDocMeta(b *strings.Builder, label, value string, color bool) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if color {
		b.WriteString(fmt.Sprintf("%s%s:%s %s\n", ui.Bold, label, ui.Reset, value))
		return
	}
	b.WriteString(fmt.Sprintf("%s: %s\n", label, value))
}

func searchExcerpt(content string) string {
	content = cleanDocComment(content)
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var parts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if len(parts) == 0 && isHeadingLine(line) {
			continue
		}
		parts = append(parts, line)
		if len(strings.Join(parts, " ")) >= 320 {
			break
		}
	}

	return truncateRunes(strings.Join(parts, " "), 360)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func isHeadingLine(line string) bool {
	switch strings.TrimSpace(line) {
	case "Overview", "Discussion":
		return true
	default:
		return false
	}
}
