package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
)

type cpuStat struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (c cpuStat) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuStat) active() uint64 {
	return c.total() - c.idle - c.iowait
}

type rawCPUStat struct {
	total  cpuStat
	ncores int
}

type sysInfo struct {
	cpuPercent float64
	cpuCores   int
	memUsed    uint64
	memTotal   uint64
	swapUsed   uint64
	swapTotal  uint64
	diskUsed   uint64
	diskTotal  uint64
	loadAvg    [3]float64
}

func readRawCPU() (rawCPUStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return rawCPUStat{}, err
	}
	defer f.Close()

	var result rawCPUStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		var s cpuStat
		ptrs := []*uint64{&s.user, &s.nice, &s.system, &s.idle, &s.iowait, &s.irq, &s.softirq}
		for i, p := range ptrs {
			*p, _ = strconv.ParseUint(fields[i+1], 10, 64)
		}
		if len(fields) > 8 {
			s.steal, _ = strconv.ParseUint(fields[8], 10, 64)
		}
		if fields[0] == "cpu" {
			result.total = s
		} else {
			result.ncores++
		}
	}
	return result, scanner.Err()
}

func collectSysInfo(prev rawCPUStat) (sysInfo, rawCPUStat, error) {
	curr, err := readRawCPU()
	if err != nil {
		return sysInfo{}, curr, err
	}

	var info sysInfo
	info.cpuCores = curr.ncores

	dt := curr.total.total() - prev.total.total()
	if dt > 0 {
		currActive := curr.total.active()
		prevActive := prev.total.active()
		if currActive >= prevActive {
			pct := float64(currActive-prevActive) / float64(dt) * 100
			if pct > 100 {
				pct = 100
			}
			info.cpuPercent = pct
		}
	}

	if mf, err2 := os.Open("/proc/meminfo"); err2 == nil {
		defer mf.Close()
		var memFree, buffers, cached, sReclaimable, swapFree uint64
		scanner := bufio.NewScanner(mf)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			val *= 1024
			switch fields[0] {
			case "MemTotal:":
				info.memTotal = val
			case "MemFree:":
				memFree = val
			case "Buffers:":
				buffers = val
			case "Cached:":
				cached = val
			case "SReclaimable:":
				sReclaimable = val
			case "SwapTotal:":
				info.swapTotal = val
			case "SwapFree:":
				swapFree = val
			}
		}
		used := info.memTotal - memFree - buffers - cached - sReclaimable
		if used <= info.memTotal {
			info.memUsed = used
		}
		info.swapUsed = info.swapTotal - swapFree
	}

	if la, err3 := os.ReadFile("/proc/loadavg"); err3 == nil {
		fields := strings.Fields(string(la))
		for i := 0; i < 3 && i < len(fields); i++ {
			info.loadAvg[i], _ = strconv.ParseFloat(fields[i], 64)
		}
	}

	var stat syscall.Statfs_t
	if err4 := syscall.Statfs("/", &stat); err4 == nil {
		bsize := uint64(stat.Bsize)
		info.diskTotal = stat.Blocks * bsize
		if stat.Blocks >= stat.Bfree {
			info.diskUsed = (stat.Blocks - stat.Bfree) * bsize
		}
	}

	return info, curr, nil
}

func renderSysInfo(info sysInfo, width int) string {
	const barW = 30
	var sb strings.Builder

	sb.WriteString(styleUnderline.Render("System Utilization") + "\n\n")

	coreStr := ""
	if info.cpuCores > 0 {
		coreStr = fmt.Sprintf("  (%d cores)", info.cpuCores)
	}
	fmt.Fprintf(&sb, "%-5s  %s %5.1f%%%s\n",
		"CPU", sysBar(info.cpuPercent/100, barW), info.cpuPercent, coreStr)

	writeRow := func(label string, used, total uint64) {
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		extra := ""
		if total > 0 {
			extra = fmt.Sprintf("  %s / %s", fmtBytes(used), fmtBytes(total))
		}
		fmt.Fprintf(&sb, "%-5s  %s %5.1f%%%s\n", label, sysBar(pct/100, barW), pct, extra)
	}

	writeRow("MEM", info.memUsed, info.memTotal)
	if info.swapTotal > 0 {
		writeRow("SWAP", info.swapUsed, info.swapTotal)
	}
	writeRow("DISK", info.diskUsed, info.diskTotal)

	fmt.Fprintf(&sb, "\nLoad avg  1m: %.2f  5m: %.2f  15m: %.2f\n",
		info.loadAvg[0], info.loadAvg[1], info.loadAvg[2])

	return sb.String()
}

func sysBar(fraction float64, width int) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	var color lipgloss.Color
	switch {
	case fraction > 0.8:
		color = lipgloss.Color("1") // red
	case fraction > 0.6:
		color = lipgloss.Color("214") // orange
	default:
		color = lipgloss.Color("2") // green
	}
	return lipgloss.NewStyle().Foreground(color).Render(bar)
}

func fmtBytes(b uint64) string {
	switch {
	case b >= 1 << 30:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	case b >= 1 << 20:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	case b >= 1 << 10:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
