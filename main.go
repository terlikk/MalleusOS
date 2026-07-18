// MalleusOS — punkt startowy binarki `malleus`.
// Uruchamia agenta metryk (pętla próbkująca co sekundę)
// i serwer HTTP z API REST + strumieniem SSE.
package main

import (
	"flag"
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"malleus/agent"
	"malleus/server"
	"malleus/storage"
	"malleus/web"
)

// Wersja wpisana na sztywno; przy wydaniach będzie podmieniana.
const version = "0.1.0-dev"

func main() {
	// flag.String definiuje opcję linii poleceń: ./malleus -addr :8443
	addr := flag.String("addr", ":8443", "adres, na którym nasłuchuje serwer HTTP")
	interval := flag.Duration("interval", time.Second, "co ile zbierać próbkę metryk")
	dataDir := flag.String("data", "data", "katalog na bazę danych")
	flag.Parse()

	kolektor := agent.NewCollector()

	// Historia próbek: najpierw próbujemy SQLite (przetrwa restart);
	// gdy się nie uda (np. katalog tylko do odczytu), łagodnie
	// schodzimy do bufora w pamięci — system działa dalej.
	var magazyn agent.Store
	if db, err := storage.Open(filepath.Join(*dataDir, "malleus.db")); err != nil {
		log.Printf("SQLite niedostępne (%v) — historia tylko w pamięci", err)
		magazyn = agent.NewHistory(2*time.Hour, *interval)
	} else {
		defer db.Close()
		// Sprzątanie w tle: trzymamy 24h historii, czyścimy co godzinę.
		go db.PruneLoop(24*time.Hour, time.Hour)
		magazyn = db
	}

	// `go` uruchamia pętlę zbierania w tle (goroutine) —
	// serwer HTTP działa równolegle i tylko czyta wyniki.
	go kolektor.Run(magazyn, *interval)

	// Panel WWW wkompilowany w binarkę; fs.Sub "wchodzi" do dist/,
	// żeby index.html był w korzeniu serwowanych plików.
	panel, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(kolektor, magazyn, version, panel)
	log.Printf("malleus %s — panel pod http://localhost%s", version, *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
