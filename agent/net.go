package agent

import (
	"os"
	"strconv"
	"strings"
)

// readNetCounters sumuje liczniki bajtów ze wszystkich
// interfejsów sieciowych w /proc/net/dev, pomijając lo
// (pętlę lokalną — ruch komputera do samego siebie).
//
// Format wiersza:
//   eth0: 12345 67 0 0 0 0 0 0 54321 89 0 0 0 0 0 0
// gdzie pierwsza liczba to bajty odebrane, dziewiąta — wysłane.
func readNetCounters() (rx, tx uint64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		iface := strings.TrimSpace(name)
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx
}
