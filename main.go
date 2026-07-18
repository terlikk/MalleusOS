// MalleusOS — punkt startowy binarki `malleus`.
// Uruchamia agenta metryk (pętla próbkująca co sekundę)
// i serwer HTTP z API REST + strumieniem SSE.
package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"malleus/agent"
	"malleus/docker"
	"malleus/mdns"
	"malleus/server"
	"malleus/storage"
	"malleus/web"
)

// Wersja domyślnie "dev"; przy budowaniu wydania podmienia ją
// linker (flaga -X w Makefile) na numer taga, np. v0.1.0.
var version = "0.1.0-dev"

func main() {
	// flag.String definiuje opcję linii poleceń: ./malleus -addr :8443
	addr := flag.String("addr", ":8443", "adres, na którym nasłuchuje serwer HTTP")
	interval := flag.Duration("interval", time.Second, "co ile zbierać próbkę metryk")
	dataDir := flag.String("data", "data", "katalog na bazę danych")
	dockerSock := flag.String("docker-sock", "/var/run/docker.sock", "socket Dockera (albo atrapy fakedocker)")
	proxyAddr := flag.String("proxy", ":80", "adres reverse proxy dla adresów .malleus.local (puste = wyłącz)")
	mdnsOn := flag.Bool("mdns", true, "rozgłaszaj nazwy .malleus.local w sieci lokalnej (mDNS)")
	flag.Parse()

	kolektor := agent.NewCollector()

	// Historia próbek: najpierw próbujemy SQLite (przetrwa restart);
	// gdy się nie uda (np. katalog tylko do odczytu), łagodnie
	// schodzimy do bufora w pamięci — system działa dalej.
	// Baza obsługuje też logowanie: bez niej hasła nie ma gdzie
	// trzymać, więc auth zostaje wyłączone (z ostrzeżeniem).
	var magazyn agent.Store
	var auth server.AuthStore
	var projekty server.ProjectStore
	if db, err := storage.Open(filepath.Join(*dataDir, "malleus.db")); err != nil {
		log.Printf("SQLite niedostępne (%v) — historia w pamięci, LOGOWANIE WYŁĄCZONE", err)
		magazyn = agent.NewHistory(2*time.Hour, *interval)
	} else {
		defer db.Close()
		// Sprzątanie w tle: trzymamy 24h historii, czyścimy co godzinę.
		go db.PruneLoop(24*time.Hour, time.Hour)
		magazyn = db
		auth = db
		projekty = db
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

	srv := server.New(server.Config{
		Col:     kolektor,
		Hist:    magazyn,
		Version: version,
		Assets:  panel,
		Docker:   docker.New(*dockerSock),
		Auth:     auth,
		Projects: projekty,
	})

	// Ładne adresy: reverse proxy na :80 (filmy.malleus.local →
	// aplikacja) + responder mDNS. Awaria żadnego z nich nie
	// zatrzymuje panelu — logujemy i działamy dalej.
	if *proxyAddr != "" {
		go func() {
			p := &http.Server{
				Addr:              *proxyAddr,
				Handler:           srv.ProxyHandler(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			if err := p.ListenAndServe(); err != nil {
				log.Printf("proxy %s: %v — adresy .malleus.local nie będą działać (port 80 wymaga roota)", *proxyAddr, err)
			}
		}()
	}
	if *mdnsOn {
		go func() {
			if err := mdns.Serve(); err != nil {
				log.Printf("mdns: %v — nazwy .malleus.local nie będą rozgłaszane", err)
			}
		}()
	}
	log.Printf("malleus %s — panel pod http://localhost%s", version, *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
