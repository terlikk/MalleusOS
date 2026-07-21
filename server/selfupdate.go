package server

// Samo-aktualizacja MalleusOS: panel pyta GitHub o najnowsze
// wydanie, a na życzenie pobiera nową binarkę, podmienia siebie
// na dysku i restartuje się w miejscu (exec) — sesje i dane
// przeżywają, bo mieszkają w SQLite.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const repo = "terlikk/MalleusOS"

// handleUpdateCheck porównuje naszą wersję z najnowszym wydaniem.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	defer res.Body.Close()
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if res.StatusCode != 200 || json.NewDecoder(res.Body).Decode(&rel) != nil || rel.Tag == "" {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	// Porównanie bez przedrostka v: wydanie v0.2.0 vs wersja 0.1.0.
	current := strings.TrimPrefix(s.version, "v")
	latest := strings.TrimPrefix(rel.Tag, "v")
	writeJSON(w, http.StatusOK, map[string]any{
		"current":   s.version,
		"latest":    rel.Tag,
		"available": latest != current && !strings.Contains(current, "dev"),
	})
}

// handleUpdateApply pobiera binarkę z najnowszego wydania,
// podmienia plik wykonywalny i restartuje proces.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	exe, err := os.Executable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	url := fmt.Sprintf(
		"https://github.com/%s/releases/latest/download/malleus-linux-%s",
		repo, runtime.GOARCH)
	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Get(url)
	if err != nil || res.StatusCode != 200 {
		if res != nil {
			res.Body.Close()
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "nie udało się pobrać nowej wersji z GitHuba"})
		return
	}
	defer res.Body.Close()

	// Nowa binarka najpierw ląduje obok starej, potem atomowa
	// podmiana przez rename — w razie zerwania pobierania stara
	// wersja zostaje nietknięta.
	tmp := exe + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	f.Close()
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "message": "zaktualizowano — restartuję się, odśwież stronę za chwilę",
	})

	// Restart po chwili, żeby odpowiedź zdążyła dojść. Nowy proces
	// dostaje te same argumenty; stary kończy się czysto.
	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}()
}
