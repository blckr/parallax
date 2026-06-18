package container

import (
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
		// Any error (binary missing, user namespaces disabled, daemon issues)
		// is silently skipped — systemd-managed containers are picked up via
		// podman-*.service units in the systemd backend instead.
		return nil, nil
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
	return runSystemctl(action, c.Unit)
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
