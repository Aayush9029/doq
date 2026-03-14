package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Aayush9029/doq/internal/docs"
	"github.com/Aayush9029/doq/internal/render"
	"github.com/Aayush9029/doq/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type docsSearchResultsMsg []docs.SearchResult
type docsEntryMsg *docs.Entry

type DocsModel struct {
	input   textinput.Model
	query   string
	results []docs.SearchResult
	cursor  int

	preview viewport.Model
	viewing bool
	entry   *docs.Entry

	width  int
	height int
	ready  bool

	lastKeystroke time.Time
	searching     bool
	err           string
}

func NewDocsModel() *DocsModel {
	ti := textinput.New()
	ti.Placeholder = "Search Apple docs..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 60

	vp := viewport.New(80, 20)

	return &DocsModel{
		input:   ti,
		preview: vp,
	}
}

func (m *DocsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *DocsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.preview.Width = msg.Width
		m.preview.Height = msg.Height - 4
		m.input.Width = msg.Width - 4
		m.ready = true
		return m, nil

	case docsSearchResultsMsg:
		m.results = []docs.SearchResult(msg)
		m.searching = false
		m.cursor = 0
		m.err = ""
		return m, nil

	case docsEntryMsg:
		m.entry = (*docs.Entry)(msg)
		m.viewing = true
		m.preview.SetContent(render.FormatDocEntry(m.entry, m.width-2, true))
		m.preview.GotoTop()
		return m, nil

	case searchErrorMsg:
		m.err = string(msg)
		m.searching = false
		return m, nil

	case debounceMsg:
		if time.Since(m.lastKeystroke) >= 150*time.Millisecond && m.query != "" {
			m.searching = true
			return m, m.doSearch(m.query)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	newQuery := m.input.Value()
	if newQuery != m.query {
		m.query = newQuery
		m.lastKeystroke = time.Now()
		if m.query == "" {
			m.results = nil
			m.cursor = 0
		} else {
			cmds = append(cmds, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
				return debounceMsg{}
			}))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *DocsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.viewing {
			m.viewing = false
			m.entry = nil
			return m, nil
		}
		if m.query != "" {
			m.input.SetValue("")
			m.query = ""
			m.results = nil
			m.cursor = 0
			return m, nil
		}
		return m, tea.Quit

	case "q":
		if !m.viewing && m.query == "" {
			return m, tea.Quit
		}

	case "up", "k":
		if !m.viewing && len(m.results) > 0 {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}
		if m.viewing {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}

	case "down", "j":
		if !m.viewing && len(m.results) > 0 {
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			return m, nil
		}
		if m.viewing {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}

	case "enter", "right":
		if !m.viewing && len(m.results) > 0 && m.cursor < len(m.results) {
			return m, m.loadEntry(m.results[m.cursor])
		}

	case "left":
		if m.viewing {
			m.viewing = false
			m.entry = nil
			return m, nil
		}

	case "tab":
		if len(m.results) > 0 {
			if m.viewing {
				m.viewing = false
				m.entry = nil
			} else {
				return m, m.loadEntry(m.results[m.cursor])
			}
			return m, nil
		}
	}

	if !m.viewing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		newQuery := m.input.Value()
		if newQuery != m.query {
			m.query = newQuery
			m.lastKeystroke = time.Now()
			if m.query == "" {
				m.results = nil
				m.cursor = 0
			} else {
				cmd = tea.Batch(cmd, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
					return debounceMsg{}
				}))
			}
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}

func (m *DocsModel) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, err := docs.Search(ctx, query, docs.SearchOptions{
			Limit:       50,
			OmitContent: true,
		})
		if err != nil {
			return searchErrorMsg(err.Error())
		}
		return docsSearchResultsMsg(results)
	}
}

func (m *DocsModel) loadEntry(result docs.SearchResult) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		entry, err := docs.Get(ctx, result.ID)
		if err != nil {
			return searchErrorMsg(err.Error())
		}
		return docsEntryMsg(entry)
	}
}

func (m *DocsModel) View() string {
	if !m.ready {
		return ""
	}
	if m.viewing {
		return m.viewPreview()
	}
	return m.viewSearch()
}

func (m *DocsModel) viewSearch() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(titleStyle.Render("⚡ doq docs"))
	b.WriteString("  ")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("-", m.width)))
	b.WriteString("\n")

	switch {
	case m.err != "":
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errStyle.Render("✗" + m.err))
		b.WriteString("\n")
	case m.searching:
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("⏳ Searching..."))
		b.WriteString("\n")
	case len(m.results) > 0:
		maxVisible := m.height - 4
		if maxVisible < 1 {
			maxVisible = 1
		}

		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.results) {
			end = len(m.results)
		}

		for i := start; i < end; i++ {
			line := render.FormatDocSearchLine(m.results[i], true)
			if i == m.cursor {
				b.WriteString(ui.Bold + "> " + ui.Reset + line)
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	case m.query != "":
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("No results"))
		b.WriteString("\n")
	default:
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("Type to search Apple developer docs semantically"))
		b.WriteString("\n")
	}

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	footerText := "up/down navigate  enter preview  q quit"
	if len(m.results) > 0 {
		footerText = fmt.Sprintf("%d results  %s", len(m.results), footerText)
	}

	lines := strings.Count(b.String(), "\n")
	for range m.height - lines - 1 {
		b.WriteString("\n")
	}
	b.WriteString(footer.Render(footerText))
	return b.String()
}

func (m *DocsModel) viewPreview() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	if m.entry != nil {
		header := m.entry.Title
		if header == "" {
			header = m.entry.ID
		}
		b.WriteString(titleStyle.Render("⚡ " + header))
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("-", m.width)))
	b.WriteString("\n")
	b.WriteString(m.preview.View())
	b.WriteString("\n")

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(footer.Render("esc back  up/down scroll  q quit"))
	return b.String()
}
