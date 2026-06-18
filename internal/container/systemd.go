package container

import (
	"fmt"
	"os/exec"
	"strings"
)

func getSystemdContainers() ([]Container, error) {
	// Which container units are currently active?
	runningMap := make(map[string]bool)
	activeOut, _ := exec.Command(
		"systemctl", "list-units", "--state=active", "--no-legend", "--no-pager", "container@*",
	).Output()
	for line := range strings.SplitSeq(string(activeOut), "\n") {
		for _, f := range strings.Fields(line) {
			if strings.HasPrefix(f, "container@") {
				runningMap[f] = true
				break
			}
		}
	}

	// All configured container unit-files.
	out, err := exec.Command(
		"systemctl", "list-unit-files", "container@*", "--no-legend", "--no-pager",
	).Output()
	if err != nil {
		return nil, err
	}

	var containers []Container
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		fullUnit := fields[0]
		at := strings.Index(fullUnit, "@")
		dot := strings.Index(fullUnit, ".")
		if at == -1 || dot == -1 || dot <= at {
			continue
		}
		name := fullUnit[at+1 : dot]
		if name == "" {
			continue
		}
		status := "stopped"
		if runningMap[fullUnit] {
			status = "running"
		}
		containers = append(containers, Container{
			Name:    name,
			Status:  status,
			Runtime: "systemd",
			Unit:    fullUnit,
		})
	}
	return containers, nil
}

func toggleSystemd(c Container) error {
	action := "start"
	if c.Status == "running" {
		action = "stop"
	}
	return exec.Command("systemctl", action, c.Unit).Run()
}

func getStatusSystemd(c Container) string {
	if c.Status != "running" {
		return fmt.Sprintf("Container %q is stopped.\n\nPress 's' to start it.", c.Name)
	}
	out, err := exec.Command("machinectl", "status", c.Name, "--no-pager").Output()
	if err != nil {
		return fmt.Sprintf("Error fetching status: %v", err)
	}
	return string(out)
}
