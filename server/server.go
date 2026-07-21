// Pakiet server wystawia HTTP API panelu MalleusOS.
// Sam stdlib: net/http do serwera, encoding/json do odpowiedzi.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"malleus/agent"
	"malleus/docker"
)

// Config zbiera wszystko, czego serwer potrzebuje do działania —
// jedna struktura zamiast coraz dłuższej listy argumentów.
type Config struct {
	Col     *agent.Collector
	Hist    agent.Store
	Version string
	Assets  fs.FS          // zbudowany panel WWW (embed)
	Docker   *docker.Client // klient socketu Dockera
	Auth     AuthStore      // nil = logowanie wyłączone (brak bazy)
	Projects ProjectStore   // nil = projekty wyłączone (brak bazy)
}

// Server spina kolektor metryk, historię i Dockera z routingiem HTTP.
type Server struct {
	col      *agent.Collector
	hist     agent.Store
	version  string
	assets   fs.FS
	docker   *docker.Client
	auth     AuthStore
	projects ProjectStore
	mux      *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{
		col: cfg.Col, hist: cfg.Hist, version: cfg.Version,
		assets: cfg.Assets, docker: cfg.Docker, auth: cfg.Auth,
		projects: cfg.Projects,
		mux:      http.NewServeMux(),
	}

	// Wzorzec "GET /ścieżka" (Go 1.22+) ogranicza trasę do jednej
	// metody HTTP — inne dostaną automatycznie 405.
	//
	// Otwarte bez logowania: health (dla monitoringu), status/setup/
	// login (inaczej nie dałoby się zalogować) i pliki panelu
	// (przeglądarka musi pobrać ekran logowania).
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/v1/setup", s.handleSetup)
	s.mux.HandleFunc("POST /api/v1/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/logout", s.handleLogout)
	s.mux.HandleFunc("GET /", s.handleRoot)

	// Wszystko poniżej wymaga ważnej sesji.
	s.mux.HandleFunc("GET /api/v1/system", s.protect(s.handleSystem))
	s.mux.HandleFunc("GET /api/v1/metrics", s.protect(s.handleMetrics))
	s.mux.HandleFunc("GET /api/v1/metrics/history", s.protect(s.handleHistory))
	s.mux.HandleFunc("GET /api/v1/stream", s.protect(s.handleStream))
	s.mux.HandleFunc("GET /api/v1/containers", s.protect(s.handleContainers))
	s.mux.HandleFunc("POST /api/v1/containers/{id}/{action}", s.protect(s.handleContainerAction))
	s.mux.HandleFunc("GET /api/v1/containers/{id}/logs", s.protect(s.handleContainerLogs))
	s.mux.HandleFunc("GET /api/v1/catalog", s.protect(s.handleCatalog))
	s.mux.HandleFunc("POST /api/v1/catalog/{id}/install", s.protect(s.handleCatalogInstall))
	s.mux.HandleFunc("POST /api/v1/catalog/{id}/uninstall", s.protect(s.handleCatalogUninstall))
	s.mux.HandleFunc("GET /api/v1/catalog/{id}/backup", s.protect(s.handleCatalogBackup))
	s.mux.HandleFunc("POST /api/v1/catalog/{id}/files", s.protect(s.handleCatalogFiles))
	s.mux.HandleFunc("POST /api/v1/catalog/custom", s.protect(s.handleCustomCreate))
	s.mux.HandleFunc("DELETE /api/v1/catalog/custom/{id}", s.protect(s.handleCustomDelete))
	s.mux.HandleFunc("POST /api/v1/catalog/{id}/update", s.protect(s.handleCatalogUpdate))
	s.mux.HandleFunc("GET /api/v1/update", s.protect(s.handleUpdateCheck))
	s.mux.HandleFunc("POST /api/v1/update", s.protect(s.handleUpdateApply))
	s.mux.HandleFunc("GET /api/v1/projects", s.protect(s.handleProjects))
	s.mux.HandleFunc("POST /api/v1/projects", s.protect(s.handleProjectCreate))
	s.mux.HandleFunc("DELETE /api/v1/projects/{name}", s.protect(s.handleProjectDelete))
	return s
}

// newHTTPServer buduje serwer z naszymi limitami czasu.
// Limit tylko na nagłówki — SSE i logi żyją godzinami.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// ListenAndServe blokuje aż do zamknięcia serwera.
func (s *Server) ListenAndServe(addr string) error {
	return newHTTPServer(addr, s.mux).ListenAndServe()
}

// writeJSON serializuje v i wysyła z właściwym nagłówkiem.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleRoot serwuje pliki panelu z pamięci binarki.
// Ścieżki bez rozszerzenia (przyszłe podstrony) dostają index.html —
// klasyczne zachowanie dla aplikacji jednostronicowych.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	if _, err := fs.Stat(s.assets, p); err != nil {
		if strings.Contains(path.Base(p), ".") {
			http.NotFound(w, r)
			return
		}
		p = "index.html"
	}
	if _, err := fs.Stat(s.assets, p); err != nil {
		// Panel niezbudowany (świeży klon bez `make web`) —
		// API działa, więc mówimy o tym wprost zamiast rzucać 404.
		fmt.Fprintf(w, "MalleusOS %s — API pod /api/v1/ (panel: uruchom `make web` i przebuduj)\n", s.version)
		return
	}
	http.ServeFileFS(w, r, s.assets, p)
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
