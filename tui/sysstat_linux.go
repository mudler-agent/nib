package tui

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// systemCPUTimes returns the machine's cumulative busy and total CPU time in
// jiffies, from the aggregate "cpu" line of /proc/stat. Idle and iowait count
// as not busy. The HUD turns two samples into a utilization percentage.
func systemCPUTimes() (busy, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	return parseCPULine(sc.Text())
}

// parseCPULine parses "cpu  user nice system idle iowait irq softirq steal ...".
// guest and guest_nice are already folded into user and nice, so they are
// not added again.
func parseCPULine(line string) (busy, total uint64, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var idle uint64
	for i, f := range fields[1:] {
		if i >= 8 {
			break
		}
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		total += v
		if i == 3 || i == 4 { // idle, iowait
			idle += v
		}
	}
	return total - idle, total, true
}

// systemMemory returns used and total physical memory in bytes, from
// /proc/meminfo. Used is MemTotal - MemAvailable: page cache the kernel can
// reclaim is not counted, matching what free(1) calls "used".
func systemMemory() (used, total int64, ok bool) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	return parseMeminfo(string(b))
}

func parseMeminfo(s string) (used, total int64, ok bool) {
	var avail int64 = -1
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = kb * 1024
		case "MemAvailable:":
			avail = kb * 1024
		}
	}
	if total <= 0 || avail < 0 {
		return 0, 0, false
	}
	return total - avail, total, true
}
