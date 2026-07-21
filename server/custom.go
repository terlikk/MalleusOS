package server

// Własne szablony aplikacji: użytkownik wypełnia w panelu prosty
// formularz (nazwa, obraz z Docker Huba, porty, foldery na dane…),
// a my zapisujemy z tego zwykły szablon YAML w katalogu danych.
// Od tej chwili apka wygląda i działa jak każda z katalogu:
// instalacja, aktualizacja, kopia zapasowa — wszystko za darmo.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"malleus/catalog"
)

// handleCustomCreate przyjmuje formularz i zapisuje szablon.
func (s *Server) handleCustomCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Tagline string `json:"tagline"`
		Image   string `json:"image"`
		WebPort int    `json:"webPort"`
		// Dodatkowe porty "host:kontener" albo "host:kontener/udp"
		Ports []string `json:"ports"`
		// Ścieżki w kontenerze na dane, np. "/data" — każda dostaje
		// nazwany wolumen, więc łapie się na kopie zapasowe
		Volumes []string `json:"volumes"`
		// Zmienne środowiskowe "NAZWA=wartość"
		Env []string `json:"env"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nieprawidłowe dane formularza"})
		return
	}

	fail := func(msg string) { writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg}) }

	body.Name = strings.TrimSpace(body.Name)
	body.Image = strings.TrimSpace(body.Image)
	if body.Name == "" || body.Image == "" {
		fail("podaj nazwę i obraz Dockera (np. nginx:latest)")
		return
	}
	id := catalog.Slug(body.Name)
	if id == "" {
		fail("nazwa musi zawierać choć jedną literę lub cyfrę")
		return
	}

	// Unikalność id względem CAŁEGO katalogu (wbudowane + własne).
	apps, err := catalog.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for _, a := range apps {
		if a.ID == id {
			fail("aplikacja o nazwie „" + body.Name + "” już jest w katalogu — wybierz inną nazwę")
			return
		}
	}

	app := catalog.App{
		ID:      id,
		Name:    body.Name,
		Tagline: strings.TrimSpace(body.Tagline),
		Image:   body.Image,
		WebPort: body.WebPort,
	}
	if app.Tagline == "" {
		app.Tagline = "własna aplikacja"
	}
	// Port WWW dostaje mapowanie i subdomenę (slug → slug.malleus.local)
	if app.WebPort > 0 {
		if app.WebPort > 65535 {
			fail("port WWW musi być z zakresu 1–65535")
			return
		}
		app.Subdomain = id
		app.Ports = append(app.Ports, catalog.Port{Host: app.WebPort, Container: app.WebPort})
	}

	for _, p := range body.Ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		proto := ""
		if rest, ok := strings.CutSuffix(p, "/udp"); ok {
			proto, p = "udp", rest
		}
		var host, cont int
		// "8080:80" = port na serwerze : port w kontenerze;
		// samo "8080" znaczy ten sam numer po obu stronach
		if strings.Contains(p, ":") {
			_, err = fmt.Sscanf(p, "%d:%d", &host, &cont)
		} else {
			_, err = fmt.Sscanf(p, "%d", &host)
			cont = host
		}
		if err != nil || host < 1 || host > 65535 || cont < 1 || cont > 65535 {
			fail("port „" + p + "” — wpisz np. 8080:80 albo 25565:25565/udp")
			return
		}
		app.Ports = append(app.Ports, catalog.Port{Host: host, Container: cont, Protocol: proto})
	}

	for _, v := range body.Volumes {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "/") {
			fail("folder danych „" + v + "” musi być ścieżką w kontenerze, np. /data")
			return
		}
		name := catalog.Slug(v)
		if name == "" {
			name = fmt.Sprintf("vol%d", len(app.Volumes)+1)
		}
		app.Volumes = append(app.Volumes, catalog.Volume{Name: name, Path: v, Opis: "dane aplikacji"})
	}

	for _, e := range body.Env {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		name, value, ok := strings.Cut(e, "=")
		if !ok || strings.TrimSpace(name) == "" {
			fail("zmienna „" + e + "” — wpisz w formie NAZWA=wartość")
			return
		}
		app.Env = append(app.Env, catalog.EnvVar{
			Name: strings.TrimSpace(name), Value: value, Opis: "ustawienie własne",
		})
	}

	if err := catalog.Save(app); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleCustomDelete usuwa szablon użytkownika z katalogu.
// Zainstalowaną aplikację trzeba najpierw odinstalować — inaczej
// zostałby kontener-sierota bez wpisu w katalogu.
func (s *Server) handleCustomDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app, err := catalog.ByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !app.Custom {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "wbudowanych szablonów nie da się usunąć"})
		return
	}
	if _, err := s.docker.InspectContainer(r.Context(), id); err == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "najpierw odinstaluj aplikację (usuń), potem szablon"})
		return
	}
	if err := catalog.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
