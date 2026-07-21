package server

// Kopie zapasowe: GET /api/v1/catalog/{id}/backup pakuje wolumeny
// aplikacji (świat Minecrafta, sejf Vaultwarden…) w jeden plik
// tar.gz i wysyła go do przeglądarki jako pobieranie. Plik można
// trzymać gdziekolwiek — to zwykłe archiwum.

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"malleus/catalog"
)

func (s *Server) handleCatalogBackup(w http.ResponseWriter, r *http.Request) {
	app, err := catalog.ByID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if len(app.Volumes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ta aplikacja nie ma wolumenów z danymi"})
		return
	}

	// Najpierw ustal ścieżki wszystkich wolumenów — jeśli któryś
	// nie istnieje, lepiej powiedzieć to PRZED rozpoczęciem pobierania.
	type vol struct{ name, path string }
	var vols []vol
	for _, v := range app.Volumes {
		volName := fmt.Sprintf("malleus-%s-%s", app.ID, v.Name)
		mp, err := s.docker.VolumeMountpoint(r.Context(), volName)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "nie znalazłem wolumenu " + volName + " — czy aplikacja jest zainstalowana?",
			})
			return
		}
		vols = append(vols, vol{name: v.Name, path: mp})
	}

	filename := fmt.Sprintf("malleus-%s-kopia-%s.tar.gz",
		app.ID, time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	for _, v := range vols {
		// Każdy wolumen ląduje w archiwum w swoim podkatalogu,
		// np. data/… config/… — przy odtwarzaniu wiadomo co skąd.
		if err := tarTree(tw, v.path, v.name); err != nil {
			// Nagłówki już poszły — możemy tylko przerwać strumień;
			// przeglądarka zgłosi nieudane pobieranie.
			return
		}
	}
	tw.Close()
	gz.Close()
}

// tarTree dopisuje do archiwum katalog src pod nazwą prefix/…
// (jak tarDir z wdrażania projektów, ale z prefiksem i po symlinkach).
func tarTree(tw *tar.Writer, src, prefix string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		name := prefix
		if rel != "." {
			name = filepath.ToSlash(filepath.Join(prefix, rel))
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, _ = os.Readlink(path)
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
