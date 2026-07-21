package agent

import (
	"fmt"
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
	// GatewayIP to adres routera (bramy domyślnej) — panel używa
	// go np. do linku "otwórz panel routera" przy Pi-hole.
	GatewayIP string `json:"gatewayIp,omitempty"`
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
	info.GatewayIP = defaultGateway()
	return info
}

// defaultGateway czyta adres routera z /proc/net/route.
// Format: wiersze z kolumnami Iface Destination Gateway…, liczby
// szesnastkowo w odwróconej kolejności bajtów (little-endian).
// Trasa domyślna ma Destination 00000000.
func defaultGateway() string {
	for _, line := range strings.Split(readTrim("/proc/net/route"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		raw, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil || raw == 0 {
			continue
		}
		return fmt.Sprintf("%d.%d.%d.%d",
			raw&0xff, (raw>>8)&0xff, (raw>>16)&0xff, (raw>>24)&0xff)
	}
	return ""
}
