package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleStream wysyła metryki strumieniem SSE (Server-Sent Events).
//
// SSE to zwykłe, długo otwarte połączenie HTTP, którym serwer
// dosyła zdarzenia tekstowe w formacie:
//
//	event: metrics
//	data: {...json...}
//	<pusta linia>
//
// Przeglądarka obsługuje to wbudowaną klasą EventSource — łącznie
// z automatycznym wznawianiem po zerwaniu. Dla danych płynących
// tylko w jedną stronę (serwer→klient) SSE jest prostsze i lżejsze
// niż WebSocket; WebSocket zostawiamy na terminal (dwukierunkowy).
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming nieobsługiwany", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		// Kontekst żądania gaśnie, gdy klient zamknie kartę —
		// wtedy kończymy pętlę i zwalniamy goroutine.
		case <-r.Context().Done():
			return
		case <-ticker.C:
			data, err := json.Marshal(s.col.Latest())
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", data)
			// Flush wypycha zdarzenie natychmiast, zamiast czekać
			// aż uzbiera się pełny bufor odpowiedzi.
			flusher.Flush()
		}
	}
}
