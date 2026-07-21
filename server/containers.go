package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"malleus/docker"
)

// handleContainers zwraca listę kontenerów wraz z chwilowym
// zużyciem CPU/RAM (pomiary lecą równolegle — każdy trwa ~1 s).
// Gdy Docker nie działa — łagodna degradacja: available=false.
func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	list, err := s.docker.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available":  false,
			"containers": []docker.Container{},
		})
		return
	}

	type entry struct {
		docker.Container
		docker.ContainerStats
	}
	out := make([]entry, len(list))
	var wg sync.WaitGroup
	for i, c := range list {
		out[i].Container = c
		if c.State != "running" {
			continue
		}
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			// Błąd pomiaru = zera w tabeli; nie psujemy listy.
			stats, _ := s.docker.Stats(r.Context(), id)
			out[i].ContainerStats = stats
		}(i, c.ID)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, map[string]any{
		"available":  true,
		"containers": out,
	})
}

// handleContainerAction wykonuje start/stop/restart.
// {id} i {action} wyciągamy z wzorca trasy (r.PathValue).
func (s *Server) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	id, action := r.PathValue("id"), r.PathValue("action")
	if err := s.docker.Action(r.Context(), id, action); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleContainerLogs strumieniuje logi kontenera przez SSE:
// ostatnie 200 linii i wszystko, co pojawi się później.
func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming nieobsługiwany", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	// Czytanie z Dockera dzieje się w osobnej goroutine, która
	// wrzuca linie do kanału; my tu tylko przepisujemy je do SSE.
	lines := make(chan docker.LogLine, 64)
	errc := make(chan error, 1)
	go func() {
		errc <- s.docker.StreamLogs(r.Context(), r.PathValue("id"), "200", lines)
		close(lines)
	}()

	for line := range lines {
		data, err := json.Marshal(line)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Kanał zamknięty. Jeśli to nie klient się rozłączył,
	// przekaż powód (np. "no such container") do przeglądarki.
	if err := <-errc; err != nil && r.Context().Err() == nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
	}
}
