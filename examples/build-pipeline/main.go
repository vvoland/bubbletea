package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	dimStyle     = lipgloss.NewStyle().Faint(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type task struct {
	name string
	logs []string
}

type model struct {
	tasks   []task
	current int
	spinner int
	paused  bool
}

type spinnerTick struct{}
type taskFinished struct{}
type advanceTask struct{}

func newModel() model {
	integrationLogs := []string{"integration tests"}
	for i := 1; i <= 24; i++ {
		integrationLogs = append(integrationLogs, fmt.Sprintf("  PASS  integration/container/TestServiceStart/%02d", i))
	}

	return model{tasks: []task{
		{
			name: "resolve dependencies",
			logs: []string{
				"resolve dependencies",
				"  restored 42 modules from cache",
				"  verified module checksums",
			},
		},
		{
			name: "lint",
			logs: []string{
				"lint",
				"  checked 84 files",
				"  no issues found",
			},
		},
		{
			name: "unit tests",
			logs: []string{
				"unit tests",
				"  PASS  renderer       42 tests",
				"  PASS  input          31 tests",
				"  PASS  commands       18 tests",
				"  PASS  program        27 tests",
			},
		},
		{name: "integration tests", logs: integrationLogs},
	}}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(runTask(), tickSpinner())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyPressMsg:
		return m, tea.Quit

	case spinnerTick:
		if m.paused {
			return m, nil
		}
		m.spinner = (m.spinner + 1) % len(spinnerFrames)
		return m, tickSpinner()

	case taskFinished:
		m.paused = true
		printLogs := tea.Println(renderLogs(m.tasks[m.current].logs))
		return m, tea.Sequence(
			printLogs,
			tea.Tick(450*time.Millisecond, func(time.Time) tea.Msg { return advanceTask{} }),
		)

	case advanceTask:
		m.current++
		if m.current == len(m.tasks) {
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tea.Quit() })
		}
		m.paused = false
		return m, tea.Batch(runTask(), tickSpinner())
	}
	return m, nil
}

func (m model) View() tea.View {
	width := 48
	progress := m.current * width / len(m.tasks)
	bar := strings.Repeat("━", progress) + strings.Repeat("─", width-progress)

	if m.current == len(m.tasks) {
		return tea.NewView(fmt.Sprintf(
			"%s\n%s",
			successStyle.Render(fmt.Sprintf("✓ Pipeline complete (%d/%d)", len(m.tasks), len(m.tasks))),
			successStyle.Render(bar),
		))
	}

	return tea.NewView(fmt.Sprintf(
		"%s Running %s (%d/%d)\n%s",
		spinnerStyle.Render(spinnerFrames[m.spinner]),
		m.tasks[m.current].name, m.current+1, len(m.tasks),
		successStyle.Render(bar),
	))
}

func runTask() tea.Cmd {
	return tea.Tick(650*time.Millisecond, func(time.Time) tea.Msg {
		return taskFinished{}
	})
}

func tickSpinner() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTick{}
	})
}

func renderLogs(lines []string) string {
	formatted := make([]string, len(lines))
	formatted[0] = successStyle.Render("✓ " + lines[0])
	for i := 1; i < len(lines); i++ {
		formatted[i] = dimStyle.Render(lines[i])
	}
	return strings.Join(formatted, "\n")
}

func main() {
	_, _ = tea.NewProgram(newModel(), tea.WithWindowSize(80, 24)).Run()
}
