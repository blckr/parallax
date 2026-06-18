package container

import (
	"fmt"
	"os/exec"
	"strings"
)

func getDockerContainers() ([]Container, error) {
	out, err := exec.Command(
		"docker", "ps", "-a",
		"--format", "{{.Names}}\t{{.Status}}",
	).Output()
	if err != nil {
		// Docker not installed or daemon not running — silently skip.
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
			Runtime: "docker",
			Unit:    name,
		})
	}
	return containers, nil
}

func toggleDocker(c Container) error {
	action := "start"
	if c.Status == "running" {
		action = "stop"
	}
	return exec.Command("docker", action, c.Name).Run()
}

func getStatusDocker(c Container) string {
	if c.Status != "running" {
		return fmt.Sprintf("Container %q is stopped.\n\nPress 's' to start it.", c.Name)
	}
	// docker inspect --format uses Go templates; .Name starts with "/"
	const tpl = "Name:    {{slice .Name 1}}\n" +
		"Image:   {{.Config.Image}}\n" +
		"Status:  {{.State.Status}}\n" +
		"Started: {{.State.StartedAt}}\n" +
		"IP:      {{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}"
	out, err := exec.Command("docker", "inspect", "--format", tpl, c.Name).Output()
	if err != nil {
		return fmt.Sprintf("docker inspect failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}
