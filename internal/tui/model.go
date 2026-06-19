package tui

import (
	"codeberg.org/blckr/parallax/internal/container"
	"codeberg.org/blckr/parallax/internal/terminal"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type appMode int

const (
	modeNormal       appMode = iota
	modeTerminal             // keyboard input is forwarded to the PTY
	modeTerminalDone         // process exited; pane stays open until user dismisses
	modeToggle               // systemctl running in PTY for auth; auto-closes on exit
)

type detailPane int

const (
	detailStatus  detailPane = iota
	detailSysInfo            // shows local host CPU/mem/disk utilization
)

type errMsg struct{ err error }
type containersLoadedMsg []container.Container

// Model holds the full application state.
type Model struct {
	// container list
	containers []container.Container
	cursor     int
	err        error
	ready      bool

	// top-pane status viewport
	viewport   viewport.Model
	detailView detailPane

	// system utilization (refreshed every tick, no connection required)
	sysInfo sysInfo
	prevCPU rawCPUStat
	cpuEMA  float64 // exponential moving average; -1 = not yet initialized

	// window dimensions (set on first WindowSizeMsg)
	windowWidth  int
	windowHeight int

	// terminal pane state
	mode         appMode
	session      *terminal.Session
	connectedTo  string
	termLines    []string
	termViewport viewport.Model
}

func StartApp() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithFPS(10))
	_, err := p.Run()
	return err
}

func initialModel() Model {
	vp := viewport.New(60, 20)
	vp.SetContent("Select a container to see status...")
	return Model{
		containers: []container.Container{},
		viewport:   vp,
		cpuEMA:     -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			c, err := container.GetAll()
			if err != nil {
				return errMsg{err}
			}
			return containersLoadedMsg(c)
		},
		fetchSysInfoCmd(m.prevCPU),
	)
}
