package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Aayush9029/doq/internal/index"
	"github.com/Aayush9029/doq/internal/render"
	"github.com/Aayush9029/doq/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type searchResultsMsg []index.SearchResult
type symbolDetailMsg *index.FullSymbol
type searchErrorMsg string
type debounceMsg struct{}

// Model is the root TUI model.
type Model struct {
	idx *index.Index

	// Search
	input   textinput.Model
	query   string
	results []index.SearchResult
	cursor  int

	// Preview
	preview  viewport.Model
	viewing  bool
	symbol   *index.FullSymbol

	// Layout
	width  int
	height int
	ready  bool

	// Debounce
	lastKeystroke time.Time
	searching     bool
	err           string
}

// NewModel creates a new TUI model.
func NewModel(idx *index.Index) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search symbols..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 60

	vp := viewport.New(80, 20)

	return &Model{
		idx:     idx,
		input:   ti,
		preview: vp,
	}
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case searchResultsMsg:
		m.results = []index.SearchResult(msg)
		m.searching = false
		m.cursor = 0
		m.err = ""
		return m, nil

	case symbolDetailMsg:
		m.symbol = (*index.FullSymbol)(msg)
		m.viewing = true
		content := render.FormatSymbol(m.symbol, m.width-2, true)
		m.preview.SetContent(content)
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

	// Update text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Check if query changed
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

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.viewing {
			m.viewing = false
			m.symbol = nil
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
		// Fall through to text input

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
			return m, m.loadSymbol(m.results[m.cursor])
		}

	case "left":
		if m.viewing {
			m.viewing = false
			m.symbol = nil
			return m, nil
		}

	case "tab":
		if len(m.results) > 0 {
			if m.viewing {
				m.viewing = false
				m.symbol = nil
			} else {
				return m, m.loadSymbol(m.results[m.cursor])
			}
			return m, nil
		}
	}

	// Forward to text input when not viewing
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

	// Forward to viewport when viewing
	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}

func (m *Model) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results, err := m.idx.Search(ctx, query, 50)
		if err != nil {
			return searchErrorMsg(err.Error())
		}
		return searchResultsMsg(results)
	}
}

func (m *Model) loadSymbol(r index.SearchResult) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Search for precise_id through the index
		sym, err := m.idx.GetSymbolByName(ctx, r.Name)
		if err != nil {
			return searchErrorMsg(err.Error())
		}
		return symbolDetailMsg(sym)
	}
}

func (m *Model) View() string {
	if !m.ready {
		return ""
	}

	if m.viewing {
		return m.viewPreview()
	}
	return m.viewSearch()
}

func (m *Model) viewSearch() string {
	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")) // Cyan
	b.WriteString(titleStyle.Render("⚡ doq"))
	b.WriteString("  ")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Separator
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("-", m.width)))
	b.WriteString("\n")

	// Results
	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errStyle.Render("✗" + m.err))
		b.WriteString("\n")
	} else if m.searching {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("⏳ Searching..."))
		b.WriteString("\n")
	} else if len(m.results) > 0 {
		maxVisible := m.height - 4
		if maxVisible < 1 {
			maxVisible = 1
		}

		// Scroll window
		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.results) {
			end = len(m.results)
		}

		for i := start; i < end; i++ {
			r := m.results[i]
			line := render.FormatSearchResult(r, true)
			if i == m.cursor {
				b.WriteString(ui.Bold + "> " + ui.Reset + line)
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	} else if m.query != "" {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("No results"))
		b.WriteString("\n")
	} else {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(dimStyle.Render("Type to search Apple developer documentation"))
		b.WriteString("\n")
	}

	// Footer
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	footerText := "up/down navigate  enter preview  q quit"
	if len(m.results) > 0 {
		footerText = fmt.Sprintf("%d results  %s", len(m.results), footerText)
	}

	// Pad to fill remaining height
	lines := strings.Count(b.String(), "\n")
	for range m.height - lines - 1 {
		b.WriteString("\n")
	}
	b.WriteString(footer.Render(footerText))

	return b.String()
}

func (m *Model) viewPreview() string {
	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))

	if m.symbol != nil {
		b.WriteString(titleStyle.Render("⚡ " + m.symbol.Name))
	}
	b.WriteString("\n")

	// Separator
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("-", m.width)))
	b.WriteString("\n")

	// Content
	b.WriteString(m.preview.View())
	b.WriteString("\n")

	// Footer
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(footer.Render("esc back  up/down scroll  q quit"))

	return b.String()
}
