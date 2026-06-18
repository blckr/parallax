package container

import (
	"fmt"
	"os/exec"
)

// Container represents a manageable container from any backend.
type Container struct {
	Name    string
	Status  string // "running" or "stopped"
	Runtime string // "systemd" or "docker"
	Unit    string // systemd unit (e.g. container@foo.service) or docker container name
}

// GetAll returns containers from every available runtime.
// A missing binary is silently ignored; a broken daemon is surfaced as a
// placeholder entry so the user sees the error in the container list.
func GetAll() ([]Container, error) {
	sys, err := getSystemdContainers()
	if err != nil {
		return nil, fmt.Errorf("systemd: %w", err)
	}

	doc, docErr := getDockerContainers()
	if docErr != nil {
		doc = []Container{{
			Name:    fmt.Sprintf("docker error: %s", docErr),
			Status:  "error",
			Runtime: "docker",
		}}
	}

	pod, podErr := getPodmanContainers()
	if podErr != nil {
		pod = []Container{{
			Name:    fmt.Sprintf("podman error: %s", podErr),
			Status:  "error",
			Runtime: "podman",
		}}
	}

	return append(append(sys, doc...), pod...), nil
}

// ToggleCmd returns the *exec.Cmd that starts or stops the container.
// The caller is responsible for running it (e.g. via tea.ExecProcess so that
// interactive auth prompts like polkit can reach the terminal).
func ToggleCmd(c Container) *exec.Cmd {
	action := "start"
	if c.Status == "running" {
		action = "stop"
	}
	switch c.Runtime {
	case "docker":
		return exec.Command("docker", action, c.Name)
	default: // "systemd", "podman" — both managed via systemctl
		return exec.Command("systemctl", action, c.Unit)
	}
}

// GetStatus returns a human-readable status string for the detail pane.
func GetStatus(c Container) string {
	switch c.Runtime {
	case "docker":
		return getStatusDocker(c)
	case "podman":
		return getStatusPodman(c)
	default:
		return getStatusSystemd(c)
	}
}

// ShellCommand returns the argv slice used to open a shell in the container.
func ShellCommand(c Container) []string {
	switch c.Runtime {
	case "docker":
		return []string{"docker", "exec", "-it", c.Name, "/bin/sh"}
	case "podman":
		return []string{"podman", "exec", "-it", c.Name, "/bin/sh"}
	default:
		return []string{"machinectl", "shell", c.Name}
	}
}
