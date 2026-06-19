package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleUnderline = lipgloss.NewStyle().Bold(true).Underline(true)
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDim       = lipgloss.NewStyle().Faint(true)
	styleConnected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
)

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to exit.", m.err)
	}
	if !m.ready {
		return " Scanning systemd units..."
	}

	docStyle := lipgloss.NewStyle().Margin(1, 2)

	statusW, statusH := m.statusVpSize()
	topInner := statusH // inner content height of both top-section boxes (= outer - border)

	// lipgloss Width(N): N includes padding but excludes border.
	// lipgloss Height(N): same rule.

	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarOuterW-borderW).   // = 30; content = 30 - sidebarPadR(1) = 29
		Height(topInner).
		Border(lipgloss.NormalBorder()).
		PaddingRight(sidebarPadR)

	// detail pane outer width = winW - margins - sidebar outer
	// detail lipgloss Width = detail outer - border = statusW + detailPadL
	detailStyle := lipgloss.NewStyle().
		Width(statusW+detailPadL). // includes left padding; content = statusW
		Height(topInner).
		Border(lipgloss.NormalBorder()).
		PaddingLeft(detailPadL)

	styleDocker  := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))   // cyan
	styleSystemd := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))   // blue
	stylePodman  := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))   // magenta
	styleWarn    := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))  // orange

	// --- Sidebar ---
	var sb strings.Builder
	sb.WriteString(styleUnderline.Render("Containers") + "\n\n")
	for i, c := range m.containers {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		if c.Status == "error" {
			// Show only the first line, truncated to fit the sidebar.
			errLine := strings.SplitN(c.Name, "\n", 2)[0]
			const maxErrW = 22 // sidebar content - cursor - badge - space
			if len(errLine) > maxErrW {
				errLine = errLine[:maxErrW-1] + "…"
			}
			fmt.Fprintf(&sb, "%s%s %s\n", cursor, styleWarn.Render("[D]!"), errLine)
			continue
		}
		dot := styleGreen.Render("●")
		if c.Status != "running" {
			dot = styleRed.Render("●")
		}
		var rtBadge string
		switch c.Runtime {
		case "docker":
			rtBadge = styleDocker.Render("[D]")
		case "podman":
			rtBadge = stylePodman.Render("[P]")
		default:
			rtBadge = styleSystemd.Render("[S]")
		}
		fmt.Fprintf(&sb, "%s%s %s %s\n", cursor, rtBadge, dot, c.Name)
	}

	topSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Render(sb.String()),
		detailStyle.Render(m.detailContent()),
	)

	// --- Terminal pane ---
	var termSection string
	if m.terminalActive() {
		termW, termH := m.termVpSize()

		var header string
		switch m.mode {
		case modeToggle:
			header = styleDim.Render("Authenticating...  (ctrl+] to cancel)")
		case modeTerminalDone:
			header = styleDim.Render(fmt.Sprintf("Terminal: %s  [session ended — press any key to close]", m.connectedTo))
		default:
			header = styleConnected.Render(fmt.Sprintf("Terminal: %s", m.connectedTo)) +
				styleDim.Render("  (ctrl+] to disconnect)")
		}

		termPaneStyle := lipgloss.NewStyle().
			Width(termW+termPadL). // includes left padding; content = termW
			Height(termH+1).       // termH viewport rows + 1 header line
			Border(lipgloss.NormalBorder()).
			PaddingLeft(termPadL)

		termSection = "\n" + termPaneStyle.Render(header+"\n"+m.termViewport.View())
	}

	// --- Footer ---
	var footer string
	switch m.mode {
	case modeToggle:
		footer = "\n" + styleDim.Render(" Type password if prompted  •  ctrl+]: Cancel")
	case modeTerminal:
		footer = "\n" + styleDim.Render(" ctrl+]: Disconnect  •  j/k: Navigate Containers")
	case modeTerminalDone:
		footer = "\n" + styleDim.Render(" Any key: Close terminal pane")
	default:
		footer = "\n" + styleDim.Render(" j/k: Navigate  •  s: Start/Stop  •  Enter: Connect  •  i: Toggle View  •  PgUp/PgDn: Scroll  •  q: Quit")
	}

	return docStyle.Render(topSection + termSection + footer)
}

func (m Model) detailContent() string {
	if m.detailView == detailSysInfo {
		w, _ := m.statusVpSize()
		return renderSysInfo(m.sysInfo, w)
	}
	return m.viewport.View()
}
