package router

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// GeoUpdateTime returns the most recent modification time among non-symlink .dat files in dir.
func GeoUpdateTime(dir string) (time.Time, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, fmt.Errorf("read dat dir: %w", err)
	}
	var latest time.Time
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			continue // skip symlinks
		}
		if !strings.HasSuffix(e.Name(), ".dat") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	if latest.IsZero() {
		return time.Time{}, fmt.Errorf("no .dat files found in %s", dir)
	}
	return latest, nil
}

// ProcessUptime returns how long the process named procName has been running.
// It reads /proc/*/comm to find the PID, then calculates uptime via /proc/<pid>/stat.
func ProcessUptime(procName string) (time.Duration, error) {
	pid, err := findPIDByName(procName)
	if err != nil {
		return 0, err
	}

	// /proc/uptime: "system_uptime_seconds idle_seconds"
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read uptime: %w", err)
	}
	systemUptimeSec, err := strconv.ParseFloat(strings.Fields(string(uptimeData))[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse uptime: %w", err)
	}

	// /proc/<pid>/stat field 22 (0-indexed 21): process start time in clock ticks since boot
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, fmt.Errorf("read stat: %w", err)
	}
	// Field 2 is the comm in parens "(name)" — skip past it to avoid spaces in process names
	statStr := string(statData)
	rp := strings.LastIndex(statStr, ")")
	if rp < 0 {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}
	fields := strings.Fields(statStr[rp+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("not enough fields in /proc/stat")
	}
	// field index 20 after ") " = starttime (field 22 in spec, 0-indexed from after comm)
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse starttime: %w", err)
	}

	const clkTck = 100 // standard for embedded Linux (CONFIG_HZ=100)
	processUptimeSec := systemUptimeSec - float64(startTicks)/clkTck
	if processUptimeSec < 0 {
		processUptimeSec = 0
	}
	return time.Duration(processUptimeSec) * time.Second, nil
}

func findPIDByName(name string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("read /proc: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) == name {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process %q not found", name)
}

type SysInfo struct {
	Uptime   time.Duration
	Load1    string
	Load5    string
	Load15   string
	MemTotal uint64
	MemAvail uint64
}

func SystemInfo() (*SysInfo, error) {
	uptime, err := readUptime()
	if err != nil {
		return nil, err
	}
	load1, load5, load15, err := readLoadAvg()
	if err != nil {
		return nil, err
	}
	memTotal, memAvail, err := readMemInfo()
	if err != nil {
		return nil, err
	}
	return &SysInfo{
		Uptime:   uptime,
		Load1:    load1,
		Load5:    load5,
		Load15:   load15,
		MemTotal: memTotal,
		MemAvail: memAvail,
	}, nil
}

func readUptime() (time.Duration, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read uptime: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty /proc/uptime")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse uptime: %w", err)
	}
	return time.Duration(secs) * time.Second, nil
}

func readLoadAvg() (load1, load5, load15 string, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "", "", "", fmt.Errorf("read loadavg: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return "", "", "", fmt.Errorf("unexpected /proc/loadavg format")
	}
	return fields[0], fields[1], fields[2], nil
}

func readMemInfo() (total, avail uint64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read meminfo: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = val * 1024
		case "MemAvailable:":
			avail = val * 1024
		}
	}
	return total, avail, nil
}
