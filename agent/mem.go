package agent

import (
	"os"
	"strconv"
	"strings"
)

// readMem czyta /proc/meminfo — listę wierszy w rodzaju
// "MemTotal:  16302456 kB" — i składa z nich MemStat.
//
// "Zajęte" liczymy jako Total − Available, bo MemAvailable to
// najuczciwsza miara: uwzględnia, że cache można w każdej chwili
// oddać programom (samo "wolne" byłoby zaniżone).
func readMem() MemStat {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemStat{}
	}

	vals := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		vals[name] = v // wartości w meminfo są w kB
	}

	m := MemStat{
		TotalKB:     vals["MemTotal"],
		AvailableKB: vals["MemAvailable"],
		CachedKB:    vals["Cached"],
		SwapTotalKB: vals["SwapTotal"],
	}
	if m.TotalKB >= m.AvailableKB {
		m.UsedKB = m.TotalKB - m.AvailableKB
	}
	if vals["SwapTotal"] >= vals["SwapFree"] {
		m.SwapUsedKB = vals["SwapTotal"] - vals["SwapFree"]
	}
	if m.TotalKB > 0 {
		m.UsedPercent = 100 * float64(m.UsedKB) / float64(m.TotalKB)
	}
	return m
}
