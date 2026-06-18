package container

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func getPodmanContainers() ([]Container, error) {
	out, err := exec.Command(
		"podman", "ps", "-a",
		"--format", "{{.Names}}\t{{.Status}}",
	).Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var containers []Container
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		status := "stopped"
		if strings.HasPrefix(parts[1], "Up") {
			status = "running"
		}
		containers = append(containers, Container{
			Name:    name,
			Status:  status,
			Runtime: "podman",
			Unit:    "podman-" + name + ".service",
		})
	}
	return containers, nil
}

// togglePodman uses systemctl so that NixOS-managed units stay consistent.
func togglePodman(c Container) error {
	action := "start"
	if c.Status == "running" {
		action = "stop"
	}
	return exec.Command("systemctl", action, c.Unit).Run()
}

func getStatusPodman(c Container) string {
	if c.Status == "error" {
		return c.Name
	}
	if c.Status != "running" {
		return fmt.Sprintf("Container %q is stopped.\n\nPress 's' to start it.", c.Name)
	}
	const tpl = "Name:    {{slice .Name 1}}\n" +
		"Image:   {{.Config.Image}}\n" +
		"Status:  {{.State.Status}}\n" +
		"Started: {{.State.StartedAt}}\n" +
		"IP:      {{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}"
	out, err := exec.Command("podman", "inspect", "--format", tpl, c.Name).Output()
	if err != nil {
		return fmt.Sprintf("podman inspect failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}
