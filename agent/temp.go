package agent

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// readTemps zbiera temperatury z dwóch miejsc w /sys:
//
//  1. /sys/class/hwmon/hwmon*/ — czujniki sprzętowe (procesor,
//     dysk NVMe, płyta główna). Wartości w tysięcznych stopnia.
//  2. /sys/class/thermal/thermal_zone*/ — strefy termiczne,
//     częste na ARM (Raspberry Pi) — używane jako uzupełnienie.
//
// Na maszynach wirtualnych czujników może nie być wcale —
// wtedy zwracamy pustą listę i nic się nie psuje.
func readTemps() []TempStat {
	var temps []TempStat

	// --- hwmon ---
	hwmons, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, dir := range hwmons {
		chip := readTrim(filepath.Join(dir, "name")) // np. "coretemp"
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, input := range inputs {
			milli, err := strconv.ParseInt(readTrim(input), 10, 64)
			if err != nil {
				continue
			}
			// tempN_input → tempN_label (etykieta jest opcjonalna)
			labelFile := strings.TrimSuffix(input, "_input") + "_label"
			label := readTrim(labelFile)
			name := chip
			if label != "" {
				name = chip + " " + label
			}
			temps = append(temps, TempStat{
				Label:   name,
				Celsius: float64(milli) / 1000,
			})
		}
	}

	// --- thermal_zone (tylko gdy hwmon nic nie dał) ---
	if len(temps) == 0 {
		zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
		for _, dir := range zones {
			milli, err := strconv.ParseInt(readTrim(filepath.Join(dir, "temp")), 10, 64)
			if err != nil {
				continue
			}
			label := readTrim(filepath.Join(dir, "type"))
			if label == "" {
				label = filepath.Base(dir)
			}
			temps = append(temps, TempStat{
				Label:   label,
				Celsius: float64(milli) / 1000,
			})
		}
	}
	return temps
}

// readTrim czyta mały plik i obcina białe znaki; przy błędzie
// zwraca pusty łańcuch — wygodne przy opcjonalnych plikach w /sys.
func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
