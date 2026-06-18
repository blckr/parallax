package tui

import (
	"regexp"
	"strings"
	"time"

	"codeberg.org/blckr/parallax/internal/container"
	"codeberg.org/blckr/parallax/internal/terminal"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Message types

type tickMsg time.Time

type ptyOutputMsg []byte
type ptyExitMsg struct{}

type sessionStartedMsg struct {
	session       *terminal.Session
	containerName string
}
type sessionErrorMsg struct{ err error }
type toggleDoneMsg struct{ err error }

// ansiRegex strips ANSI/VT100 escape sequences from terminal output.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b[PX^_].*?\x1b\\|\x1b\][^\x07]*\x07|\x1b[^[\x9b]`)

// keyMap maps BubbleTea key names to the raw bytes sent to the PTY.
var keyMap = map[string][]byte{
	"enter":     {'\r'},
	"backspace": {'\x7f'},
	"tab":       {'\t'},
	"esc":       {'\x1b'},
	" ":         {' '},
	"ctrl+a":    {'\x01'},
	"ctrl+b":    {'\x02'},
	"ctrl+c":    {'\x03'},
	"ctrl+d":    {'\x04'},
	"ctrl+e":    {'\x05'},
	"ctrl+f":    {'\x06'},
	"ctrl+g":    {'\x07'},
	"ctrl+h":    {'\x08'},
	"ctrl+j":    {'\x0a'},
	"ctrl+k":    {'\x0b'},
	"ctrl+l":    {'\x0c'},
	"ctrl+n":    {'\x0e'},
	"ctrl+o":    {'\x0f'},
	"ctrl+p":    {'\x10'},
	"ctrl+q":    {'\x11'},
	"ctrl+r":    {'\x12'},
	"ctrl+s":    {'\x13'},
	"ctrl+t":    {'\x14'},
	"ctrl+u":    {'\x15'},
	"ctrl+v":    {'\x16'},
	"ctrl+w":    {'\x17'},
	"ctrl+x":    {'\x18'},
	"ctrl+y":    {'\x19'},
	"ctrl+z":    {'\x1a'},
	"up":        {'\x1b', '[', 'A'},
	"down":      {'\x1b', '[', 'B'},
	"right":     {'\x1b', '[', 'C'},
	"left":      {'\x1b', '[', 'D'},
	"pgup":      {'\x1b', '[', '5', '~'},
	"pgdown":    {'\x1b', '[', '6', '~'},
	"home":      {'\x1b', '[', 'H'},
	"end":       {'\x1b', '[', 'F'},
	"delete":    {'\x1b', '[', '3', '~'},
	"f1":        {'\x1b', 'O', 'P'},
	"f2":        {'\x1b', 'O', 'Q'},
	"f3":        {'\x1b', 'O', 'R'},
	"f4":        {'\x1b', 'O', 'S'},
	"f5":        {'\x1b', '[', '1', '5', '~'},
	"f6":        {'\x1b', '[', '1', '7', '~'},
	"f7":        {'\x1b', '[', '1', '8', '~'},
	"f8":        {'\x1b', '[', '1', '9', '~'},
	"f9":        {'\x1b', '[', '2', '0', '~'},
	"f10":       {'\x1b', '[', '2', '1', '~'},
	"f11":       {'\x1b', '[', '2', '3', '~'},
	"f12":       {'\x1b', '[', '2', '4', '~'},
}

// checkSystemCmd polls all backends for the current container list.
func checkSystemCmd() tea.Msg {
	c, err := container.GetAll()
	if err != nil {
		return errMsg{err}
	}
	return containersLoadedMsg(c)
}

func waitForNextTick() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// toggleContainer starts or stops the selected container.
func toggleContainer(c container.Container) tea.Cmd {
	return func() tea.Msg {
		return toggleDoneMsg{container.Toggle(c)}
	}
}

// connectToContainer spawns a PTY session for the given container.
func connectToContainer(c container.Container, rows, cols int) tea.Cmd {
	return func() tea.Msg {
		sess, err := terminal.NewSession(container.ShellCommand(c), rows, cols)
		if err != nil {
			return sessionErrorMsg{err}
		}
		return sessionStartedMsg{session: sess, containerName: c.Name}
	}
}

// readFromPTY issues one blocking read from the PTY and returns the bytes as a message.
// Call again after each ptyOutputMsg to keep the stream flowing.
func readFromPTY(sess *terminal.Session) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := sess.Read(buf)
		if err != nil {
			return ptyExitMsg{}
		}
		return ptyOutputMsg(buf[:n])
	}
}

// processTermOutput appends raw PTY bytes to the display line buffer.
// It strips ANSI escape sequences and handles CR/LF/BS.
func processTermOutput(lines []string, data []byte) []string {
	if len(lines) == 0 {
		lines = []string{""}
	}
	raw := string(data)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = ansiRegex.ReplaceAllString(raw, "")

	for _, ch := range raw {
		switch ch {
		case '\n':
			lines = append(lines, "")
		case '\r':
			lines[len(lines)-1] = ""
		case '\x07': // bell – ignore
		case '\x08': // backspace
			l := lines[len(lines)-1]
			if len(l) > 0 {
				lines[len(lines)-1] = l[:len(l)-1]
			}
		default:
			if ch >= 0x20 {
				lines[len(lines)-1] += string(ch)
			}
		}
	}

	const maxLines = 2000
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

// Update handles all incoming messages and mutates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m = m.recalculateLayout()
		return m, nil

	case containersLoadedMsg:
		m.containers = []container.Container(msg)
		m.ready = true
		m = m.updateStatusContent()
		return m, waitForNextTick()

	case errMsg:
		m.err = msg.err
		return m, nil

	case tickMsg:
		return m, checkSystemCmd

	case sessionStartedMsg:
		m.session = msg.session
		m.connectedTo = msg.containerName
		m.mode = modeTerminal
		m.termLines = []string{""}
		// Recalculate first so the status viewport shrinks before we size the term viewport.
		m = m.recalculateLayout()
		termW, termH := m.termVpSize()
		m.termViewport = viewport.New(termW, termH)
		return m, readFromPTY(m.session)

	case sessionErrorMsg:
		m.viewport.SetContent("Connection failed: " + msg.err.Error())
		return m, nil

	case ptyOutputMsg:
		m.termLines = processTermOutput(m.termLines, msg)
		m.termViewport.SetContent(strings.Join(m.termLines, "\n"))
		m.termViewport.GotoBottom()
		return m, readFromPTY(m.session)

	case ptyExitMsg:
		// Keep the pane open so the user can read any output/error before dismissing.
		if m.session != nil {
			m.session.Close()
			m.session = nil
		}
		m.mode = modeTerminalDone
		return m, nil

	case toggleDoneMsg:
		if msg.err != nil {
			m.viewport.SetContent("Toggle failed: " + msg.err.Error())
		}
		return m, checkSystemCmd

	case tea.KeyMsg:
		keyStr := msg.String()

		// In "done" mode any key closes the pane and restores normal layout.
		if m.mode == modeTerminalDone {
			m.disconnectSession()
			m = m.recalculateLayout()
			return m, nil
		}

		if m.mode == modeTerminal {
			// ctrl+] is the escape hatch to leave terminal mode
			if keyStr == "ctrl+]" {
				m.disconnectSession()
				m = m.recalculateLayout()
				return m, nil
			}
			// Forward everything else to the PTY
			if m.session != nil {
				if b, ok := keyMap[keyStr]; ok {
					m.session.Write(b) //nolint:errcheck
				} else if len(keyStr) == 1 {
					m.session.Write([]byte(keyStr)) //nolint:errcheck
				}
			}
			return m, nil
		}

		// Normal mode navigation
		switch keyStr {
		case "q", "ctrl+c":
			if m.session != nil {
				m.session.Close()
			}
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m = m.updateStatusContent()
			}
		case "down", "j":
			if m.cursor < len(m.containers)-1 {
				m.cursor++
				m = m.updateStatusContent()
			}

		case "s":
			if len(m.containers) > 0 {
				selected := m.containers[m.cursor]
				if selected.Status != "error" {
					return m, toggleContainer(selected)
				}
			}

		case "enter":
			if len(m.containers) > 0 {
				selected := m.containers[m.cursor]
				if selected.Status != "running" {
					break
				}
				// Temporarily switch mode so termVpSize() returns terminal-mode dimensions.
				m.mode = modeTerminal
				termW, termH := m.termVpSize()
				m.mode = modeNormal
				return m, connectToContainer(selected, termH, termW)
			}
		}
	}

	// Let the status viewport handle scroll events in normal mode
	if m.mode == modeNormal {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// disconnectSession tears down the active PTY session and returns to normal mode.
func (m *Model) disconnectSession() {
	if m.session != nil {
		m.session.Close()
		m.session = nil
	}
	m.mode = modeNormal
	m.connectedTo = ""
	m.termLines = nil
}

// updateStatusContent refreshes the status viewport for the currently selected container.
func (m Model) updateStatusContent() Model {
	if len(m.containers) > 0 {
		selected := m.containers[m.cursor]
		m.viewport.SetContent(container.GetStatus(selected))
	}
	return m
}

// recalculateLayout updates viewport dimensions after a window resize.
func (m Model) recalculateLayout() Model {
	statusW, statusH := m.statusVpSize()
	m.viewport.Width = statusW
	m.viewport.Height = statusH

	if m.terminalActive() {
		termW, termH := m.termVpSize()
		m.termViewport.Width = termW
		m.termViewport.Height = termH
		m.termViewport.SetContent(strings.Join(m.termLines, "\n"))
		m.termViewport.GotoBottom()
		if m.session != nil {
			m.session.Resize(termH, termW) //nolint:errcheck
		}
	}
	return m
}

// Layout constants.
// lipgloss.Margin(1,2) adds 1 row top+bottom and 2 cols left+right.
const (
	sidebarOuterW = 32 // sidebar total outer width  (content 29 + padR 1 + border 2)
	sidebarPadR   = 1
	detailPadL    = 1
	termPadL      = 1
	marginH       = 4 // horizontal margin (2 left + 2 right)
	marginV       = 2 // vertical margin (1 top + 1 bottom)
	footerRows    = 2 // one blank line ("\n") + one text line
	borderH       = 2 // top + bottom border per pane
	borderW       = 2 // left + right border per pane
)

// winH/winW return safe window dimensions.
func (m Model) winH() int {
	if m.windowHeight == 0 {
		return 40
	}
	return m.windowHeight
}
func (m Model) winW() int {
	if m.windowWidth == 0 {
		return 120
	}
	return m.windowWidth
}

// panelH returns the total rows available for pane boxes (excludes margins and footer).
func (m Model) panelH() int {
	return m.winH() - marginV - footerRows
}

// terminalActive reports whether the terminal pane is currently being shown.
func (m Model) terminalActive() bool {
	return m.mode == modeTerminal || m.mode == modeTerminalDone
}

// topOuterH is the outer height (including border) of the top two-column section.
func (m Model) topOuterH() int {
	ph := m.panelH()
	if m.terminalActive() {
		// panelH = topOuterH + 1(blank line) + termOuterH
		// allocate 55 % to the top section
		return (ph - 1) * 55 / 100
	}
	return ph
}

// termOuterH is the outer height (including border) of the terminal pane.
func (m Model) termOuterH() int {
	if !m.terminalActive() {
		return 0
	}
	return m.panelH() - 1 - m.topOuterH()
}

// statusVpSize returns (width, height) of the status viewport content area.
func (m Model) statusVpSize() (width, height int) {
	// width: full window minus margins, sidebar, detail border, and detail left-padding
	w := max(m.winW()-marginH-sidebarOuterW-borderW-detailPadL, 10)
	// height: top section outer height minus its border
	h := max(m.topOuterH()-borderH, 1)
	return w, h
}

// termVpSize returns (width, height) of the terminal viewport content area.
func (m Model) termVpSize() (width, height int) {
	// width: full window minus margins, terminal border, and left-padding
	w := max(m.winW()-marginH-borderW-termPadL, 10)
	// height: terminal outer height minus border and one header line
	h := max(m.termOuterH()-borderH-1, 1)
	return w, h
}
