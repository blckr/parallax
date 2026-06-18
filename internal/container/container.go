package container

import "fmt"

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

// Toggle starts a stopped container or stops a running one.
func Toggle(c Container) error {
	switch c.Runtime {
	case "docker":
		return toggleDocker(c)
	case "podman":
		return togglePodman(c)
	default:
		return toggleSystemd(c)
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
