package container

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// podmanMaintenanceUnits are Podman's own internal service units, not containers.
var podmanMaintenanceUnits = map[string]bool{
	"auto-update":     true,
	"clean-transient": true,
	"prune":           true,
	"restart":         true,
}

func getSystemdContainers() ([]Container, error) {
	nspawn, err := getNspawnContainers()
	if err != nil {
		return nil, err
	}
	podman := getPodmanUnitContainers()
	return append(nspawn, podman...), nil
}

// getNspawnContainers detects systemd-nspawn containers (container@*.service).
func getNspawnContainers() ([]Container, error) {
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

// getPodmanUnitContainers detects podman containers managed as podman-*.service
// systemd units (e.g. NixOS virtualisation.oci-containers with Podman backend).
// This works without rootless user namespaces.
func getPodmanUnitContainers() []Container {
	runningMap := make(map[string]bool)
	activeOut, _ := exec.Command(
		"systemctl", "list-units", "--state=active", "--no-legend", "--no-pager", "podman-*.service",
	).Output()
	for line := range strings.SplitSeq(string(activeOut), "\n") {
		for _, f := range strings.Fields(line) {
			if strings.HasPrefix(f, "podman-") && strings.HasSuffix(f, ".service") {
				runningMap[f] = true
				break
			}
		}
	}

	out, _ := exec.Command(
		"systemctl", "list-unit-files", "podman-*.service", "--no-legend", "--no-pager",
	).Output()

	var containers []Container
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		fullUnit := fields[0]
		if !strings.HasPrefix(fullUnit, "podman-") || !strings.HasSuffix(fullUnit, ".service") {
			continue
		}
		name := fullUnit[len("podman-") : len(fullUnit)-len(".service")]
		// Skip template units (e.g. kube@) and Podman's own maintenance services.
		if name == "" || strings.HasSuffix(name, "@") || podmanMaintenanceUnits[name] {
			continue
		}
		status := "stopped"
		if runningMap[fullUnit] {
			status = "running"
		}
		containers = append(containers, Container{
			Name:    name,
			Status:  status,
			Runtime: "podman",
			Unit:    fullUnit,
		})
	}
	return containers
}

// runSystemctl runs a systemctl command in a detached session so that polkit
// cannot reach the TUI's controlling terminal for interactive authentication.
// Any output (including auth-failure messages) is captured and returned as an error.
func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return err
}

func toggleSystemd(c Container) error {
	action := "start"
	if c.Status == "running" {
		action = "stop"
	}
	return runSystemctl(action, c.Unit)
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
