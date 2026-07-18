// Pakiet server wystawia HTTP API panelu MalleusOS.
// Sam stdlib: net/http do serwera, encoding/json do odpowiedzi.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"malleus/agent"
)

// Server spina kolektor metryk i historię z routingiem HTTP.
type Server struct {
	col     *agent.Collector
	hist    *agent.History
	version string
	mux     *http.ServeMux
}

func New(col *agent.Collector, hist *agent.History, version string) *Server {
	s := &Server{col: col, hist: hist, version: version, mux: http.NewServeMux()}

	// Wzorzec "GET /ścieżka" (Go 1.22+) ogranicza trasę do jednej
	// metody HTTP — inne dostaną automatycznie 405.
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/system", s.handleSystem)
	s.mux.HandleFunc("GET /api/v1/metrics", s.handleMetrics)
	s.mux.HandleFunc("GET /api/v1/metrics/history", s.handleHistory)
	s.mux.HandleFunc("GET /api/v1/stream", s.handleStream)
	s.mux.HandleFunc("GET /", s.handleRoot)
	return s
}

// ListenAndServe blokuje aż do zamknięcia serwera.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.mux,
		// Limit na nagłówki chroni przed wiszącymi połączeniami.
		// Celowo brak limitu na całą odpowiedź — SSE żyje godzinami.
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

// writeJSON serializuje v i wysyła z właściwym nagłówkiem.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "MalleusOS %s — API pod /api/v1/, panel WWW w budowie\n", s.version)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.version,
	})
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, agent.ReadSystemInfo())
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.col.Latest())
}

// handleHistory obsługuje ?range=15m|1h|2h (domyślnie 15m).
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "15m"
	}
	d, err := time.ParseDuration(rng)
	if err != nil || d <= 0 || d > 2*time.Hour {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "range musi być czasem od 1s do 2h, np. 15m albo 1h",
		})
		return
	}
	// Maksymalnie ~360 punktów — tyle wystarcza każdemu wykresowi.
	writeJSON(w, http.StatusOK, s.hist.Since(d, 360))
}
