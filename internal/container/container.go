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
// A missing or broken Docker daemon is silently ignored.
func GetAll() ([]Container, error) {
	sys, err := getSystemdContainers()
	if err != nil {
		return nil, fmt.Errorf("systemd: %w", err)
	}
	doc, _ := getDockerContainers() // non-fatal if Docker is absent
	return append(sys, doc...), nil
}

// Toggle starts a stopped container or stops a running one.
func Toggle(c Container) error {
	if c.Runtime == "docker" {
		return toggleDocker(c)
	}
	return toggleSystemd(c)
}

// GetStatus returns a human-readable status string for the detail pane.
func GetStatus(c Container) string {
	if c.Runtime == "docker" {
		return getStatusDocker(c)
	}
	return getStatusSystemd(c)
}

// ShellCommand returns the argv slice used to open a shell in the container.
func ShellCommand(c Container) []string {
	if c.Runtime == "docker" {
		return []string{"docker", "exec", "-it", c.Name, "/bin/sh"}
	}
	return []string{"machinectl", "shell", c.Name}
}
