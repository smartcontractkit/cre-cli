package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// progressWriter wraps an io.Writer to track download progress
type progressWriter struct {
	total      int64
	downloaded int64
	file       *os.File
	onProgress func(float64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.file.Write(p)
	pw.downloaded += int64(n)
	if pw.total > 0 && pw.onProgress != nil {
		pw.onProgress(float64(pw.downloaded) / float64(pw.total))
	}
	return n, err
}

// progressMsg is sent when download progress updates
type progressMsg float64

// progressDoneMsg is sent when download completes
type progressDoneMsg struct{}

// progressErrMsg is sent when download fails
type progressErrMsg struct{ err error }

// downloadModel is the Bubble Tea model for download progress
type downloadModel struct {
	progress progress.Model
	message  string
	percent  float64
	done     bool
	err      error
}

func (m downloadModel) Init() tea.Cmd {
	return nil
}

func (m downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case progressMsg:
		m.percent = float64(msg)
		return m, nil
	case progressDoneMsg:
		m.done = true
		return m, tea.Quit
	case progressErrMsg:
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m downloadModel) View() string {
	if m.done {
		return ""
	}
	pad := strings.Repeat(" ", 2)
	// Use ViewAs for immediate rendering without animation lag
	return "\n" + pad + DimStyle.Render(m.message) + "\n" + pad + m.progress.ViewAs(m.percent) + "\n"
}

// DownloadWithProgress downloads a file with a progress bar display.
// Returns the number of bytes downloaded and any error.
func DownloadWithProgress(resp io.ReadCloser, contentLength int64, destFile *os.File, message string) error {
	// Check if we're in a TTY
	if !term.IsTerminal(int(os.Stderr.Fd())) || contentLength <= 0 {
		// Non-TTY or unknown size: just copy without progress bar
		_, err := io.Copy(destFile, resp)
		return err
	}

	// Create progress bar with Chainlink theme colors
	prog := progress.New(
		progress.WithScaledGradient(ColorBlue600, ColorBlue300),
		progress.WithWidth(40),
	)

	m := downloadModel{
		progress: prog,
		message:  message,
	}

	// Create the Bubble Tea program
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))

	// Create progress writer
	pw := &progressWriter{
		total: contentLength,
		file:  destFile,
		onProgress: func(ratio float64) {
			p.Send(progressMsg(ratio))
		},
	}

	// Start download in goroutine
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(pw, resp)
		if err != nil {
			p.Send(progressErrMsg{err: err})
		} else {
			p.Send(progressDoneMsg{})
		}
		errCh <- err
	}()

	// Run the UI — blocks until done, error, or Ctrl+C
	finalModel, err := p.Run()
	if err != nil {
		// Bubble Tea itself failed. Close the response body to unblock the
		// download goroutine, then drain the channel so it can exit.
		resp.Close()
		<-errCh
		return err
	}

	// Close the response body so the download goroutine's io.Copy finishes.
	// For a completed download this is a no-op; for Ctrl+C it unblocks io.Copy.
	resp.Close()
	<-errCh

	// If the user pressed Ctrl+C, the model is not done — treat as cancellation
	if fm, ok := finalModel.(downloadModel); ok && !fm.done {
		return fmt.Errorf("download cancelled")
	}

	return nil
}

// FormatBytes formats bytes into human readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ProgressBar creates a simple styled progress bar string (for non-interactive use)
func ProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	empty := width - filled

	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBlue500)).Render(strings.Repeat("█", filled))
	bar += lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGray600)).Render(strings.Repeat("░", empty))

	return fmt.Sprintf("%s %.0f%%", bar, percent*100)
}
