package ui

import (
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// SpinnerStyle for the spinner character (Blue 500 - bright and visible)
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBlue500))

// globalSpinner is the shared spinner instance for the entire CLI lifecycle
var (
	globalSpinner     *Spinner
	globalSpinnerOnce sync.Once
)

// GlobalSpinner returns the shared spinner instance.
// This ensures a single spinner is used across PersistentPreRunE and command execution,
// preventing the spinner from flickering between operations.
func GlobalSpinner() *Spinner {
	globalSpinnerOnce.Do(func() {
		globalSpinner = NewSpinner()
	})
	return globalSpinner
}

// Spinner manages a terminal spinner for async operations using Bubble Tea.
// It uses reference counting to handle multiple concurrent operations -
// the spinner only stops when ALL operations complete.
type Spinner struct {
	mu        sync.Mutex
	count     int
	message   string
	program   *tea.Program
	isRunning bool
	isTTY     bool
	quitCh    chan struct{}
}

// spinnerModel is the Bubble Tea model for the spinner
type spinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
}

// Message types for the spinner
type msgUpdate string
type msgQuit struct{}

func newSpinnerModel(message string) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return spinnerModel{
		spinner: s,
		message: message,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgUpdate:
		m.message = string(msg)
		return m, nil
	case msgQuit:
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), DimStyle.Render(m.message))
}

// NewSpinner creates a new spinner instance
func NewSpinner() *Spinner {
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	return &Spinner{
		isTTY:  isTTY,
		quitCh: make(chan struct{}),
	}
}

// Start begins or continues the spinner with the given message.
// Each call to Start must be paired with a call to Stop.
// The spinner will keep running until all Start calls have been matched with Stop calls.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count++
	s.message = message

	if s.isRunning {
		// Update the message on the existing spinner
		if s.program != nil {
			s.program.Send(msgUpdate(message))
		}
		return
	}

	if !s.isTTY {
		// Non-TTY: just print the message once
		fmt.Fprintf(os.Stderr, "%s\n", DimStyle.Render(message))
		return
	}

	s.isRunning = true
	s.quitCh = make(chan struct{})

	model := newSpinnerModel(message)
	s.program = tea.NewProgram(model, tea.WithOutput(os.Stderr))

	// Run the program in a goroutine
	go func() {
		_, _ = s.program.Run()
		close(s.quitCh)
	}()
}

// Update changes the spinner message without affecting the reference count
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = message
	if s.program != nil {
		s.program.Send(msgUpdate(message))
	}
}

// Stop decrements the reference count and stops the spinner if count reaches zero
func (s *Spinner) Stop() {
	s.mu.Lock()

	if s.count > 0 {
		s.count--
	}

	if s.count == 0 && s.isRunning {
		s.isRunning = false
		if s.program != nil {
			s.program.Send(msgQuit{})
			s.mu.Unlock()
			<-s.quitCh // Wait for program to finish
			s.program = nil
			return
		}
	}

	s.mu.Unlock()
}

// StopAll forces the spinner to stop regardless of reference count
func (s *Spinner) StopAll() {
	s.mu.Lock()

	s.count = 0
	if s.isRunning {
		s.isRunning = false
		if s.program != nil {
			s.program.Send(msgQuit{})
			s.mu.Unlock()
			<-s.quitCh
			s.program = nil
			return
		}
	}

	s.mu.Unlock()
}

// Run executes a function while showing the spinner.
// This handles starting and stopping automatically.
func (s *Spinner) Run(message string, fn func() error) error {
	s.Start(message)
	err := fn()
	s.Stop()
	return err
}

// WithSpinner executes a function while showing a new spinner.
// This is a convenience function for single operations.
func WithSpinner(message string, fn func() error) error {
	s := NewSpinner()
	return s.Run(message, fn)
}

// WithSpinnerResult executes a function that returns a value while showing a spinner.
func WithSpinnerResult[T any](message string, fn func() (T, error)) (T, error) {
	s := NewSpinner()
	s.Start(message)
	result, err := fn()
	s.Stop()
	return result, err
}
