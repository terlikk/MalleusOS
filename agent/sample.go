// Pakiet agent zbiera metryki systemu prosto z /proc i /sys —
// wirtualnych plików, przez które jądro Linuksa udostępnia swój
// stan. Zero zewnętrznych zależności, sam stdlib.
package agent

// Sample to jedna "fotografia" stanu serwera z danej sekundy.
// Znaczniki `json:"..."` ustalają nazwy pól w odpowiedziach API.
type Sample struct {
	// Czas wykonania próbki w milisekundach uniksowych —
	// dokładnie ten format rozumieją wykresy w przeglądarce.
	Time int64 `json:"time"`

	CPU   CPUStat    `json:"cpu"`
	Mem   MemStat    `json:"mem"`
	Disks []DiskStat `json:"disks"`
	Net   NetStat    `json:"net"`
	Temps []TempStat `json:"temps"`
}

// CPUStat — obciążenie procesora.
// Procenty liczymy z RÓŻNIC liczników między dwiema próbkami:
// /proc/stat podaje sumaryczne "tyknięcia" od startu systemu,
// więc dopiero różnica mówi, co działo się w ostatniej sekundzie.
type CPUStat struct {
	UsagePercent float64   `json:"usagePercent"` // całość, 0–100
	PerCore      []float64 `json:"perCore"`      // osobno każdy rdzeń
	Cores        int       `json:"cores"`
	Load1        float64   `json:"load1"`  // średnie obciążenie 1 min
	Load5        float64   `json:"load5"`  // … 5 min
	Load15       float64   `json:"load15"` // … 15 min
}

// MemStat — pamięć RAM i swap, w kilobajtach (tak podaje jądro).
type MemStat struct {
	TotalKB     uint64  `json:"totalKb"`
	UsedKB      uint64  `json:"usedKb"`
	AvailableKB uint64  `json:"availableKb"`
	CachedKB    uint64  `json:"cachedKb"`
	SwapTotalKB uint64  `json:"swapTotalKb"`
	SwapUsedKB  uint64  `json:"swapUsedKb"`
	UsedPercent float64 `json:"usedPercent"`
}

// DiskStat — zajętość jednego zamontowanego systemu plików.
type DiskStat struct {
	Mount       string  `json:"mount"`  // np. /dane
	Device      string  `json:"device"` // np. /dev/sda1
	FSType      string  `json:"fstype"` // np. ext4
	TotalBytes  uint64  `json:"totalBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

// NetStat — ruch sieciowy zsumowany po interfejsach (bez lo).
// Liczniki są od startu systemu; tempo (B/s) liczymy z różnic.
type NetStat struct {
	RxBytes uint64  `json:"rxBytes"` // odebrane łącznie
	TxBytes uint64  `json:"txBytes"` // wysłane łącznie
	RxBps   float64 `json:"rxBps"`   // tempo pobierania teraz
	TxBps   float64 `json:"txBps"`   // tempo wysyłania teraz
}

// TempStat — jeden czujnik temperatury.
type TempStat struct {
	Label   string  `json:"label"` // np. "coretemp Core 0"
	Celsius float64 `json:"celsius"`
}
