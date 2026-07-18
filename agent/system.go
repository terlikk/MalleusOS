package agent

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// SystemInfo to stałe (lub wolno zmienne) dane o maszynie —
// w odróżnieniu od Sample nie trzeba ich zbierać co sekundę.
type SystemInfo struct {
	Hostname      string  `json:"hostname"`
	OS            string  `json:"os"`     // np. "Alpine Linux v3.21"
	Kernel        string  `json:"kernel"` // wersja jądra
	Arch          string  `json:"arch"`   // amd64 / arm64
	CPUModel      string  `json:"cpuModel"`
	Cores         int     `json:"cores"`
	TotalMemKB    uint64  `json:"totalMemKb"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// ReadSystemInfo składa informacje o hoście z kilku miejsc:
// /etc/os-release (nazwa dystrybucji), /proc/sys/kernel/osrelease
// (jądro), /proc/cpuinfo (model procesora), /proc/uptime.
func ReadSystemInfo() SystemInfo {
	info := SystemInfo{
		Arch:   runtime.GOARCH,
		Kernel: readTrim("/proc/sys/kernel/osrelease"),
	}
	info.Hostname, _ = os.Hostname()

	// /etc/os-release to lista KLUCZ=wartość; szukamy PRETTY_NAME.
	for _, line := range strings.Split(readTrim("/etc/os-release"), "\n") {
		if val, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			info.OS = strings.Trim(val, `"`)
			break
		}
	}

	// Model procesora: pierwszy wiersz "model name" z /proc/cpuinfo.
	// Na ARM bywa to "Model" albo go nie ma — wtedy zostaje pusto.
	for _, line := range strings.Split(readTrim("/proc/cpuinfo"), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := strings.TrimSpace(name)
		if key == "model name" || key == "Model" {
			info.CPUModel = strings.TrimSpace(rest)
			break
		}
	}

	if cur, err := readCPUCounters(); err == nil {
		info.Cores = len(cur.cores)
	}
	info.TotalMemKB = readMem().TotalKB

	if fields := strings.Fields(readTrim("/proc/uptime")); len(fields) > 0 {
		info.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
	}
	return info
}
