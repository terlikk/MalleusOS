package server

// "Moje projekty" — wdrażanie własnych aplikacji z repozytorium
// git albo pliku ZIP. Wymóg: projekt ma w korzeniu Dockerfile
// (przepis, jak zbudować obraz). Logi budowania płyną do
// przeglądarki na żywo strumieniem tekstu.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"malleus/docker"
	"malleus/storage"
)

// ProjectStore to część bazy odpowiadająca za projekty
// (implementuje ją storage.DB).
type ProjectStore interface {
	SaveProject(storage.Project) error
	ListProjects() ([]storage.Project, error)
	DeleteProject(name string) error
}

// Nazwa projektu: małe litery/cyfry/myślniki — bo robi też za
// subdomenę (nazwa.malleus.local) i nazwę kontenera.
var projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

func projContainer(name string) string { return "proj-" + name }
func projImage(name string) string     { return "malleus-proj-" + name }

// handleProjects zwraca listę projektów ze stanem kontenerów.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "projekty wymagają bazy danych"})
		return
	}
	list, err := s.projects.ListProjects()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	states := map[string]string{}
	if cs, err := s.docker.List(r.Context()); err == nil {
		for _, c := range cs {
			states[c.Name] = c.State
		}
	}

	type entry struct {
		storage.Project
		State string `json:"state"` // running / exited / "" (brak kontenera)
	}
	out := make([]entry, 0, len(list))
	for _, p := range list {
		out = append(out, entry{Project: p, State: states[projContainer(p.Name)]})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleProjectCreate przyjmuje formularz (nazwa + git URL albo
// ZIP + porty), buduje i uruchamia projekt, STRUMIENIUJĄC postęp
// jako zwykły tekst — przeglądarka czyta go na bieżąco.
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "projekty wymagają bazy danych"})
		return
	}
	// Limit 128 MB na cały upload — dość na spory projekt.
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "niepoprawny formularz: " + err.Error()})
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	gitURL := strings.TrimSpace(r.FormValue("gitUrl"))
	containerPort, _ := strconv.Atoi(r.FormValue("containerPort"))
	hostPort, _ := strconv.Atoi(r.FormValue("hostPort"))
	if hostPort == 0 {
		hostPort = containerPort
	}
	zipFile, _, zipErr := r.FormFile("zip")

	// Walidacja zanim zaczniemy strumieniować.
	switch {
	case !projectName.MatchString(name):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nazwa: małe litery, cyfry i myślniki (2–31 znaków)"})
		return
	case containerPort < 1 || containerPort > 65535:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "podaj port, na którym nasłuchuje Twoja aplikacja w kontenerze"})
		return
	case gitURL == "" && zipErr != nil:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "podaj adres repozytorium git albo wgraj plik ZIP"})
		return
	}
	if zipErr == nil {
		defer zipFile.Close()
	}

	// Od tego miejsca odpowiedź to strumień logu budowania.
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	say := func(format string, args ...any) {
		fmt.Fprintf(w, format+"\n", args...)
		if flusher != nil {
			flusher.Flush()
		}
	}
	fail := func(err error) {
		say("")
		say("BŁĄD: %v", err)
	}

	workdir, err := os.MkdirTemp("", "malleus-build-")
	if err != nil {
		fail(err)
		return
	}
	defer os.RemoveAll(workdir)
	src := filepath.Join(workdir, "src")

	// --- 1. Pozyskanie źródeł ---
	if gitURL != "" {
		say("→ Klonuję %s …", gitURL)
		if err := gitClone(r.Context(), gitURL, src, say); err != nil {
			fail(err)
			return
		}
	} else {
		say("→ Rozpakowuję ZIP…")
		tmpZip := filepath.Join(workdir, "src.zip")
		f, err := os.Create(tmpZip)
		if err != nil {
			fail(err)
			return
		}
		if _, err := io.Copy(f, zipFile); err != nil {
			f.Close()
			fail(err)
			return
		}
		f.Close()
		if err := extractZip(tmpZip, src); err != nil {
			fail(err)
			return
		}
	}

	if _, err := os.Stat(filepath.Join(src, "Dockerfile")); err != nil {
		fail(fmt.Errorf("w projekcie nie ma pliku Dockerfile — to on mówi, jak zbudować aplikację"))
		return
	}

	// --- 2. Budowanie obrazu (kontekst tar płynie przez pipe) ---
	say("→ Buduję obraz %s …", projImage(name))
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarDir(src, pw))
	}()
	if err := s.docker.BuildImage(r.Context(), projImage(name), pr, func(line string) {
		say("  %s", line)
	}); err != nil {
		fail(err)
		return
	}

	// --- 3. Kontener: stary usuwamy (przebudowa), nowy startuje ---
	say("→ Uruchamiam kontener…")
	_ = s.docker.RemoveContainer(r.Context(), projContainer(name)) // mógł nie istnieć
	err = s.docker.CreateContainer(r.Context(), projContainer(name), docker.CreateSpec{
		Image:  projImage(name),
		Ports:  []docker.PortMap{{Host: hostPort, Container: containerPort}},
		Labels: map[string]string{"malleus.project": name},
	})
	if err != nil {
		fail(err)
		return
	}
	if err := s.docker.Action(r.Context(), projContainer(name), "start"); err != nil {
		fail(err)
		return
	}

	if err := s.projects.SaveProject(storage.Project{
		Name: name, GitURL: gitURL,
		ContainerPort: containerPort, HostPort: hostPort,
		Created: time.Now().UnixMilli(),
	}); err != nil {
		fail(err)
		return
	}

	say("")
	say("✔ Gotowe! Twoja aplikacja działa pod adresem http://%s.malleus.local (port %d)", name, hostPort)
}

// handleProjectDelete kasuje kontener, obraz i wpis w bazie.
func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	if s.projects == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "projekty wymagają bazy danych"})
		return
	}
	name := r.PathValue("name")
	_ = s.docker.RemoveContainer(r.Context(), projContainer(name))
	_ = s.docker.RemoveImage(r.Context(), projImage(name))
	if err := s.projects.DeleteProject(name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// gitClone klonuje płytko (bez pełnej historii) systemowym gitem.
// Wołamy program `git`, bo w stdlib Go nie ma klienta gita —
// a jest on obecny na praktycznie każdym serwerze. Gdy go brak,
// mówimy wprost i podpowiadamy ZIP.
func gitClone(ctx context.Context, url, dest string, say func(string, ...any)) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("nie znalazłem programu git na serwerze — wgraj projekt jako ZIP")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dest)
	out, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			say("  %s", line)
		}
	}
	if err != nil {
		return fmt.Errorf("git clone nie powiódł się — sprawdź adres repozytorium")
	}
	return nil
}
