package agent

import (
	"sync"
	"time"
)

// History to bufor cykliczny (ring buffer) na próbki.
//
// Bufor cykliczny ma stałą pojemność: gdy się zapełni, najnowsza
// próbka nadpisuje najstarszą. Dzięki temu pamięć nie rośnie
// w nieskończoność — trzymamy dokładnie tyle historii, ile trzeba
// (u nas 2 godziny). Migracja na SQLite przyjdzie w etapie 2.
type History struct {
	mu   sync.RWMutex
	buf  []Sample
	next int  // indeks, w który trafi następna próbka
	full bool // czy bufor już się przekręcił
}

// NewHistory tworzy bufor mieszczący span/step próbek
// (np. 2h przy próbce co sekundę → 7200 miejsc).
func NewHistory(span, step time.Duration) *History {
	n := int(span / step)
	if n < 1 {
		n = 1
	}
	return &History{buf: make([]Sample, n)}
}

// Add dopisuje próbkę, w razie potrzeby nadpisując najstarszą.
func (h *History) Add(s Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf[h.next] = s
	h.next++
	if h.next == len(h.buf) {
		h.next = 0
		h.full = true
	}
}

// Since zwraca próbki z ostatniego okresu d, chronologicznie.
// Gdy jest ich więcej niż maxPoints, przerzedza wynik (bierze co
// n-tą) — wykres i tak nie pokaże więcej punktów niż ma pikseli.
func (h *History) Since(d time.Duration, maxPoints int) []Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Odtwarzamy kolejność chronologiczną: od najstarszej (next)
	// do najnowszej (next-1). Gdy bufor niepełny — od zera.
	var ordered []Sample
	if h.full {
		ordered = append(ordered, h.buf[h.next:]...)
		ordered = append(ordered, h.buf[:h.next]...)
	} else {
		ordered = append(ordered, h.buf[:h.next]...)
	}

	cutoff := time.Now().Add(-d).UnixMilli()
	start := 0
	for start < len(ordered) && ordered[start].Time < cutoff {
		start++
	}
	out := ordered[start:]

	if maxPoints > 0 && len(out) > maxPoints {
		stride := (len(out) + maxPoints - 1) / maxPoints
		thinned := make([]Sample, 0, maxPoints)
		for i := 0; i < len(out); i += stride {
			thinned = append(thinned, out[i])
		}
		out = thinned
	}
	return out
}
