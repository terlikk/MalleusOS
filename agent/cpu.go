package agent

import (
	"os"
	"strconv"
	"strings"
)

// cpuCounters to surowe liczniki "tyknięć" z /proc/stat.
// Wiersz "cpu" sumuje wszystkie rdzenie, "cpu0", "cpu1"… to
// poszczególne rdzenie. Kolumny: user nice system idle iowait
// irq softirq steal… Czas bezczynności = idle + iowait.
type cpuCounters struct {
	total uint64
	busy  uint64
	cores []coreCounters
}

type coreCounters struct {
	total uint64
	busy  uint64
}

func readCPUCounters() (cpuCounters, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuCounters{}, err
	}

	var c cpuCounters
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				continue
			}
			total += v
			// kolumna 4 = idle, kolumna 5 = iowait (czekanie na dysk)
			if i == 4 || i == 5 {
				idle += v
			}
		}

		if fields[0] == "cpu" {
			c.total, c.busy = total, total-idle
		} else {
			c.cores = append(c.cores, coreCounters{total: total, busy: total - idle})
		}
	}
	return c, nil
}

// cpuPercent zamienia różnicę dwóch odczytów liczników na procent.
func cpuPercent(prev, cur uint64, prevBusy, curBusy uint64) float64 {
	dTotal := cur - prev
	if dTotal == 0 {
		return 0
	}
	return 100 * float64(curBusy-prevBusy) / float64(dTotal)
}

// readLoadAvg czyta średnie obciążenie z /proc/loadavg
// (trzy pierwsze liczby: średnia z 1, 5 i 15 minut).
func readLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15
}
