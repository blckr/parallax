package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Session wraps a PTY subprocess for an in-container shell.
type Session struct {
	master *os.File
	cmd    *exec.Cmd
}

// NewSession runs shellCmd inside a PTY of the given size.
func NewSession(shellCmd []string, rows, cols int) (*Session, error) {
	cmd := exec.Command(shellCmd[0], shellCmd[1:]...)
	master, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, err
	}
	return &Session{master: master, cmd: cmd}, nil
}

// Write sends bytes to the shell's stdin.
func (s *Session) Write(b []byte) (int, error) {
	return s.master.Write(b)
}

// Read reads output bytes from the shell's stdout/stderr.
func (s *Session) Read(p []byte) (int, error) {
	return s.master.Read(p)
}

// Resize updates the PTY window dimensions.
func (s *Session) Resize(rows, cols int) error {
	return pty.Setsize(s.master, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Close kills the shell process and releases the PTY.
func (s *Session) Close() {
	s.master.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}
