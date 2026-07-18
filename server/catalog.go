package server

import (
	"fmt"
	"net/http"

	"malleus/catalog"
	"malleus/docker"
)

// handleCatalog zwraca listę aplikacji z katalogu wraz ze stanem:
// czy dana aplikacja jest już zainstalowana (kontener o nazwie
// równej jej id) i czy działa.
func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	apps, err := catalog.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Stan kontenerów — bez Dockera katalog dalej się pokazuje,
	// tylko instalacja będzie niedostępna.
	dockerOK := true
	byName := map[string]docker.Container{}
	if list, err := s.docker.List(r.Context()); err != nil {
		dockerOK = false
	} else {
		for _, c := range list {
			byName[c.Name] = c
		}
	}

	type entry struct {
		catalog.App
		Installed bool `json:"installed"`
		Running   bool `json:"running"`
	}
	out := make([]entry, 0, len(apps))
	for _, a := range apps {
		c, ok := byName[a.ID]
		out = append(out, entry{
			App:       a,
			Installed: ok,
			Running:   ok && c.State == "running",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": dockerOK,
		"apps":      out,
	})
}

// handleCatalogInstall robi całą instalację jednym żądaniem:
// pobiera obraz, tworzy kontener z portami/wolumenami/env
// z szablonu i uruchamia go. Może potrwać (pobieranie obrazu!) —
// panel pokazuje w tym czasie "instalowanie…".
func (s *Server) handleCatalogInstall(w http.ResponseWriter, r *http.Request) {
	app, err := catalog.ByID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if err := s.docker.PullImage(r.Context(), app.Image); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	spec := docker.CreateSpec{
		Image:  app.Image,
		Cmd:    app.Cmd,
		Labels: map[string]string{"malleus.app": app.ID},
	}
	for _, e := range app.Env {
		spec.Env = append(spec.Env, e.Name+"="+e.Value)
	}
	for _, p := range app.Ports {
		spec.Ports = append(spec.Ports, docker.PortMap{
			Host: p.Host, Container: p.Container, Protocol: p.Protocol,
		})
	}
	for _, v := range app.Volumes {
		// Nazwany wolumen Dockera, np. malleus-jellyfin-config —
		// tworzy się sam przy pierwszym użyciu.
		spec.Binds = append(spec.Binds,
			fmt.Sprintf("malleus-%s-%s:%s", app.ID, v.Name, v.Path))
	}

	if err := s.docker.CreateContainer(r.Context(), app.ID, spec); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := s.docker.Action(r.Context(), app.ID, "start"); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleCatalogUninstall usuwa kontener aplikacji.
// Dane (wolumeny) zostają — ponowna instalacja je podchwyci.
func (s *Server) handleCatalogUninstall(w http.ResponseWriter, r *http.Request) {
	app, err := catalog.ByID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if err := s.docker.RemoveContainer(r.Context(), app.ID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
