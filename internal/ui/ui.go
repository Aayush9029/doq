package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// ANSI color constants
const (
	Green  = "\033[32m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Blue   = "\033[34m"
	Dim    = "\033[2m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

var spinFrames = [...]string{"|", "/", "-", "\\"}

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// TermWidth returns the terminal width, defaulting to 80.
func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// Header prints a cyan bold header, only in TTY mode.
func Header(msg string) {
	if !IsTTY() {
		return
	}
	fmt.Printf("\n%s%s⚡ %s%s\n\n", Cyan, Bold, msg, Reset)
}

// Success prints a green success message.
func Success(msg string) {
	fmt.Printf("%s✓ %s%s\n", Green, msg, Reset)
}

// Error prints a red error message to stderr.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s✗ %s%s\n", Red, msg, Reset)
}

// Status prints a green indented status line.
func Status(msg string) {
	fmt.Printf("  %s→ %s%s\n", Green, msg, Reset)
}

// Waiting prints a yellow waiting message.
func Waiting(msg string) {
	fmt.Printf("%s⏳ %s%s\n", Yellow, msg, Reset)
}

// Dimf prints a dim formatted message.
func Dimf(format string, args ...any) {
	fmt.Printf("%s%s%s\n", Dim, fmt.Sprintf(format, args...), Reset)
}

// Fatalf prints an error and exits.
func Fatalf(format string, args ...any) {
	Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

// ProgressBar renders an inline ASCII progress bar that overwrites itself.
//
//	[======>           ] 42% Extracting (12/30) Swift
func ProgressBar(label string, current, total int) {
	if !IsTTY() {
		return
	}
	pct := 0
	if total > 0 {
		pct = current * 100 / total
	}

	w := TermWidth()
	suffix := fmt.Sprintf(" %3d%% %s (%d/%d)", pct, label, current, total)
	barWidth := w - len(suffix) - 4
	if barWidth < 10 {
		barWidth = 10
	}

	filled := barWidth * current / max(total, 1)
	if filled > barWidth {
		filled = barWidth
	}

	empty := barWidth - filled
	head := ""
	if filled > 0 && empty > 0 {
		filled--
		head = ">"
	}

	bar := fmt.Sprintf(" %s[%s%s%s%s%s]%s",
		Cyan,
		Green, repeatByte('=', filled), head,
		Dim, repeatByte(' ', empty),
		Reset,
	)

	fmt.Printf("\r%s%s", bar, suffix)
}

// ProgressDone clears the progress line and prints a final summary.
func ProgressDone(msg string) {
	if IsTTY() {
		fmt.Printf("\r\033[K")
	}
	Success(msg)
}

// Spinner displays an animated ASCII spinner with a label.
// Call Stop() to clear the line. Safe for concurrent use.
type Spinner struct {
	mu      sync.Mutex
	label   string
	stopCh  chan struct{}
	stopped bool
}

// NewSpinner starts a spinner with the given label.
func NewSpinner(label string) *Spinner {
	s := &Spinner{
		label:  label,
		stopCh: make(chan struct{}),
	}
	if !IsTTY() {
		fmt.Printf("[~] %s\n", label)
		return s
	}
	go s.run()
	return s
}

func (s *Spinner) run() {
	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			f := spinFrames[frame%len(spinFrames)]
			fmt.Printf("\r %s%s%s %s", Cyan, f, Reset, s.label)
			frame++
			s.mu.Unlock()
		}
	}
}

// Update changes the spinner label.
func (s *Spinner) Update(label string) {
	s.mu.Lock()
	s.label = label
	s.mu.Unlock()
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stopCh)
	if IsTTY() {
		fmt.Printf("\r\033[K")
	}
}

func repeatByte(ch byte, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
