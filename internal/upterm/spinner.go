// Copyright 2025 Upbound Inc.
// All rights reserved

package upterm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	bspinner "github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/utils/ptr"

	"github.com/upbound/up/internal/style"
)

// WrapWithSuccessSpinner adds spinners around message and run function.
func WrapWithSuccessSpinner(msg string, f func() error, printer ObjectPrinter) error {
	if bool(printer.Quiet) {
		return f()
	}

	wrap := func(_ context.Context) error {
		return f()
	}
	s := spinner.New().
		Title(msg).
		ActionWithErr(wrap).
		TitleStyle(lipgloss.NewStyle())

	if printer.Pretty {
		s = s.Style(style.UpboundRootStyle)
	} else {
		s = s.
			Accessible(true).
			Style(lipgloss.NewStyle())
	}

	err := s.Run()

	ind := "✓"
	if err != nil {
		ind = "✗"
	}
	if printer.Pretty {
		ind = style.UpboundRootStyle.Render(ind)
	}
	fmt.Printf("%s %s\n", ind, msg) //nolint:forbidigo // This is an output library.

	return err
}

// StepCounter returns the counted steps.
func StepCounter(msg string, index, total int) string {
	return fmt.Sprintf("[%d/%d]: %s", index, total, msg)
}

// MultiSpinner is a collection of independent spinners that get displayed
// together. Spinners can be dynamically added.
type MultiSpinner struct {
	spinners []*SuccessSpinner
	mu       sync.Mutex
	program  *tea.Program
}

type tickMsg time.Time

func tick(t time.Time) tea.Msg {
	return tickMsg(t)
}

// Init satisfies tea.Model.
func (m *MultiSpinner) Init() tea.Cmd {
	return tea.Tick(bspinner.Dot.FPS, tick)
}

// Update satisfies tea.Model.
func (m *MultiSpinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := msg.(tickMsg); !ok {
		return m, nil
	}

	for _, sp := range m.spinners {
		_, _ = sp.Update(msg)
	}

	return m, tea.Tick(bspinner.Dot.FPS, tick)
}

// View satisfies tea.Model.
func (m *MultiSpinner) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	views := make([]string, len(m.spinners))
	for i, sp := range m.spinners {
		views[i] = sp.View()
	}

	return strings.Join(views, "\n")
}

// Add adds a spinner to the multi-spinner.
func (m *MultiSpinner) Add(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sp := range m.spinners {
		if sp.title == title {
			// Spinner already exists.
			return
		}
	}

	m.spinners = append(m.spinners, NewSuccessSpinner(title))
}

// Success marks an existing spinner in the multi-spinner as having succeeded.
func (m *MultiSpinner) Success(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sp := range m.spinners {
		if sp.title != title {
			continue
		}
		sp.Success()
		return
	}
}

// Fail marks an existing spinner in the multi-spinner as having failed.
func (m *MultiSpinner) Fail(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sp := range m.spinners {
		if sp.title != title {
			continue
		}
		sp.Fail()
		return
	}
}

// Start starts the spinners.
func (m *MultiSpinner) Start() {
	m.program = tea.NewProgram(m,
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
	)

	go func() {
		_, _ = m.program.Run()
	}()
}

// Stop stops the spinners.
func (m *MultiSpinner) Stop() {
	if m.program == nil {
		return
	}

	// Send a final tick so we update the display.
	m.program.Send(tick(time.Now()))

	m.program.Quit()
	m.program.Wait()
}

// SuccessSpinner is a spinner that can be marked as successful or failed and
// updates its view accordingly. It is used by MultiSpinner, but can also be
// used as a standalone spinner.
type SuccessSpinner struct {
	title   string
	success *bool
	spinner bspinner.Model
	log     []string
	mu      sync.Mutex

	program *tea.Program
}

// NewSuccessSpinner returns an initialized SuccessSpinner.
func NewSuccessSpinner(msg string) *SuccessSpinner {
	return &SuccessSpinner{
		title: msg,
		spinner: bspinner.New(
			bspinner.WithSpinner(bspinner.Dot),
			bspinner.WithStyle(style.UpboundRootStyle),
		),
	}
}

// Init satisfies tea.Model.
func (ss *SuccessSpinner) Init() tea.Cmd {
	return tea.Tick(bspinner.Dot.FPS, tick)
}

// Update satisfies tea.Model.
func (ss *SuccessSpinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if _, ok := msg.(tickMsg); !ok {
		return ss, nil
	}
	ss.spinner, _ = ss.spinner.Update(ss.spinner.Tick())

	return ss, tea.Tick(bspinner.Dot.FPS, tick)
}

// View satisfies tea.Model.
func (ss *SuccessSpinner) View() string {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ind := ss.spinner.View()
	if ss.success != nil {
		ind = style.UpboundRootStyle.Render("✓")
		if !*ss.success {
			ind = style.UpboundRootStyle.Render("✗")
		}
	}

	view := fmt.Sprintf("%s %s", ind, ss.title)
	if len(ss.log) > 0 {
		view += "\n" + strings.Join(ss.log, "\n") + "\n"
	}

	return view
}

// UpdateText updates the spinner's text.
func (ss *SuccessSpinner) UpdateText(msg string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.title = msg
}

// Success marks the spinner in the multi-spinner as having succeeded.
func (ss *SuccessSpinner) Success() {
	ss.mu.Lock()
	ss.success = ptr.To(true)
	ss.mu.Unlock()

	// stop calls Update, so we have to do it oustide the lock.
	ss.stop()
}

// Fail marks an existing spinner in the multi-spinner as having failed.
func (ss *SuccessSpinner) Fail() {
	ss.mu.Lock()
	ss.success = ptr.To(false)
	ss.mu.Unlock()

	// stop calls Update, so we have to do it oustide the lock.
	ss.stop()
}

// Logf adds a formatted message to the log printed under the spinner.
func (ss *SuccessSpinner) Logf(format string, args ...any) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.log = append(ss.log, fmt.Sprintf("ℹ️ "+format, args...))
}

// Start starts the spinners.
func (ss *SuccessSpinner) Start() {
	ss.program = tea.NewProgram(ss,
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
	)

	go func() {
		_, _ = ss.program.Run()
	}()
}

// Stop stops the spinners.
func (ss *SuccessSpinner) stop() {
	if ss.program == nil {
		return
	}

	// Send a final tick so we update the display.
	ss.program.Send(tick(time.Now()))

	ss.program.Quit()
	ss.program.Wait()
}
