package server

// Narzędzia do pracy z archiwami przy wdrażaniu projektów:
// rozpakowanie ZIP-a od użytkownika i spakowanie katalogu w tar
// (format kontekstu budowania, którego wymaga Docker).

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractZip rozpakowuje archiwum do katalogu dest.
//
// Dwie pułapki, które obsługujemy:
//  1. "zip slip" — złośliwy wpis w rodzaju "../../etc/passwd"
//     mógłby wyjść poza katalog docelowy; odrzucamy takie ścieżki.
//  2. ZIP-y z GitHuba ("Download ZIP") mają wszystko w jednym
//     folderze najwyższego poziomu — wykrywamy go i pomijamy,
//     żeby Dockerfile wylądował w korzeniu.
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("nie mogę otworzyć ZIP-a: %w", err)
	}
	defer r.Close()

	// Czy wszystkie wpisy siedzą we wspólnym folderze?
	prefix := ""
	if len(r.File) > 0 {
		first, _, found := strings.Cut(r.File[0].Name, "/")
		if found {
			prefix = first + "/"
			for _, f := range r.File {
				if !strings.HasPrefix(f.Name, prefix) {
					prefix = ""
					break
				}
			}
		}
	}

	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.Clean("/"+name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("podejrzana ścieżka w archiwum: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// tarDir pakuje katalog w strumień tar (pisze do w na bieżąco).
func tarDir(dir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}
		// Pomijamy .git — historia repo nie jest częścią builda,
		// a potrafi ważyć więcej niż sam kod.
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
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
