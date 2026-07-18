package agent

import (
	"sync"
	"time"
)

// Collector zbiera próbki w pętli i pamięta ostatnią.
//
// Trzyma też poprzednie odczyty liczników (CPU, sieć), bo procenty
// i tempa liczy się z RÓŻNIC między dwiema próbkami — pojedynczy
// odczyt mówi tylko "ile od startu systemu", nie "ile teraz".
type Collector struct {
	// mu chroni pole latest przed jednoczesnym zapisem (pętla)
	// i odczytem (żądania HTTP) z różnych goroutine'ów.
	mu     sync.RWMutex
	latest Sample

	// Stan poprzedniej próbki — używany tylko przez pętlę Run,
	// więc nie wymaga blokady.
	prevCPU cpuCounters
	prevRx  uint64
	prevTx  uint64
	prevAt  time.Time
}

// NewCollector robi pierwszy, "zerowy" odczyt liczników, żeby
// pierwsza prawdziwa próbka miała się od czego odjąć.
func NewCollector() *Collector {
	c := &Collector{}
	c.prevCPU, _ = readCPUCounters()
	c.prevRx, c.prevTx = readNetCounters()
	c.prevAt = time.Now()
	return c
}

// Latest zwraca ostatnią zebraną próbkę (bezpiecznie dla wątków).
func (c *Collector) Latest() Sample {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

// Run zbiera próbkę co `every` i dopisuje ją do historii.
// Uruchamiaj w osobnej goroutine: go kolektor.Run(historia, time.Second)
func (c *Collector) Run(hist *History, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for range ticker.C {
		s := c.collect()

		c.mu.Lock()
		c.latest = s
		c.mu.Unlock()

		if hist != nil {
			hist.Add(s)
		}
	}
}

// collect składa jedną pełną próbkę. Wywoływane wyłącznie
// z pętli Run (modyfikuje stan prev*).
func (c *Collector) collect() Sample {
	now := time.Now()
	elapsed := now.Sub(c.prevAt).Seconds()

	// --- CPU: procenty z różnic liczników ---
	cur, _ := readCPUCounters()
	cpu := CPUStat{
		UsagePercent: cpuPercent(c.prevCPU.total, cur.total, c.prevCPU.busy, cur.busy),
		Cores:        len(cur.cores),
	}
	for i, core := range cur.cores {
		if i < len(c.prevCPU.cores) {
			prev := c.prevCPU.cores[i]
			cpu.PerCore = append(cpu.PerCore,
				cpuPercent(prev.total, core.total, prev.busy, core.busy))
		}
	}
	cpu.Load1, cpu.Load5, cpu.Load15 = readLoadAvg()

	// --- Sieć: tempo w bajtach na sekundę z różnic liczników ---
	rx, tx := readNetCounters()
	net := NetStat{RxBytes: rx, TxBytes: tx}
	if elapsed > 0 {
		// Liczniki mogą się cofnąć (restart interfejsu) —
		// wtedy tempo zostaje zerowe zamiast ujemnego.
		if rx >= c.prevRx {
			net.RxBps = float64(rx-c.prevRx) / elapsed
		}
		if tx >= c.prevTx {
			net.TxBps = float64(tx-c.prevTx) / elapsed
		}
	}

	// Zapamiętujemy bieżące liczniki dla następnej próbki.
	c.prevCPU = cur
	c.prevRx, c.prevTx = rx, tx
	c.prevAt = now

	return Sample{
		Time:  now.UnixMilli(),
		CPU:   cpu,
		Mem:   readMem(),
		Disks: listDisks(),
		Net:   net,
		Temps: readTemps(),
	}
}
