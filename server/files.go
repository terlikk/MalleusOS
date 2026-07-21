package server

// "Dodaj treść": serwer pobiera plik (np. paczkę .zim Kiwiksa)
// prosto do wolumenu aplikacji i restartuje ją — użytkownik nie
// dotyka terminala. Postęp płynie jako linie tekstu, tak samo
// jak przy instalacji.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"malleus/catalog"
)

// rozmiar zamienia bajty na "1.4 GB" — do statusów postępu.
func rozmiar(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%d MB", b/(1<<20))
	default:
		return fmt.Sprintf("%d KB", b/(1<<10))
	}
}

// handleCatalogFiles obsługuje POST /api/v1/catalog/{id}/files
// z ciałem {"url": "https://..."}.
func (s *Server) handleCatalogFiles(w http.ResponseWriter, r *http.Request) {
	app, err := catalog.ByID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if app.Pliki == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ta aplikacja nie przyjmuje plików z treścią"})
		return
	}

	var body struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body)
	u, err := url.Parse(strings.TrimSpace(body.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "podaj pełny link http(s) do pliku"})
		return
	}
	name := path.Base(u.Path)
	if app.Pliki.Ext != "" && !strings.HasSuffix(strings.ToLower(name), app.Pliki.Ext) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "link musi prowadzić do pliku " + app.Pliki.Ext + " (np. z library.kiwix.org)",
		})
		return
	}

	// Wolumen musi istnieć = aplikacja zainstalowana.
	volName := fmt.Sprintf("malleus-%s-%s", app.ID, app.Pliki.Volume)
	mount, err := s.docker.VolumeMountpoint(r.Context(), volName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "najpierw zainstaluj aplikację"})
		return
	}

	// Od tego miejsca statusy płyną na żywo (jak przy instalacji).
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fl, _ := w.(http.Flusher)
	say := func(msg string) {
		fmt.Fprintln(w, msg)
		if fl != nil {
			fl.Flush()
		}
	}

	say("łączenie z serwerem plików…")
	req, err := http.NewRequestWithContext(r.Context(), "GET", u.String(), nil)
	if err != nil {
		say("BŁĄD: " + err.Error())
		return
	}
	// Bez limitu czasu — paczki potrafią mieć kilkanaście GB;
	// zerwanie połączenia i tak przerwie pobieranie przez context.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		say("BŁĄD: nie udało się połączyć: " + err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		say("BŁĄD: serwer plików odpowiedział " + res.Status + " — sprawdź link")
		return
	}

	// Najpierw plik tymczasowy, po udanym pobraniu zmiana nazwy —
	// przerwane pobieranie nie zostawia połówki pliku, którą
	// aplikacja próbowałaby czytać.
	tmp := filepath.Join(mount, name+".part")
	dst := filepath.Join(mount, name)
	f, err := os.Create(tmp)
	if err != nil {
		say("BŁĄD: " + err.Error())
		return
	}

	total := res.ContentLength
	var got, lastPct, lastMB int64
	buf := make([]byte, 1<<20)
	for {
		n, rerr := res.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				// najczęstszy powód: pełna karta SD
				say("BŁĄD: zapis nie powiódł się (brak miejsca na dysku?): " + werr.Error())
				return
			}
			got += int64(n)
			if total > 0 {
				// melduj co 1% — nie zalewamy panelu tysiącami linii
				if pct := got * 100 / total; pct != lastPct {
					lastPct = pct
					say(fmt.Sprintf("pobieranie %d%% (%s z %s)", pct, rozmiar(got), rozmiar(total)))
				}
			} else if mb := got / (50 << 20); mb != lastMB {
				lastMB = mb
				say("pobrano " + rozmiar(got))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			say("BŁĄD: pobieranie przerwane: " + rerr.Error())
			return
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		say("BŁĄD: " + err.Error())
		return
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		say("BŁĄD: " + err.Error())
		return
	}

	say("restartowanie aplikacji…")
	if err := s.docker.Action(r.Context(), app.ID, "restart"); err != nil {
		say("BŁĄD: plik pobrany, ale restart się nie udał: " + err.Error())
		return
	}
	say("OK")
}
