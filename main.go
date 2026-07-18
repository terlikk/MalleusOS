// MalleusOS — punkt startowy binarki `malleus`.
// Uruchamia agenta metryk (pętla próbkująca co sekundę)
// i serwer HTTP z API REST + strumieniem SSE.
package main

import (
	"flag"
	"log"
	"time"

	"malleus/agent"
	"malleus/server"
)

// Wersja wpisana na sztywno; przy wydaniach będzie podmieniana.
const version = "0.1.0-dev"

func main() {
	// flag.String definiuje opcję linii poleceń: ./malleus -addr :8443
	addr := flag.String("addr", ":8443", "adres, na którym nasłuchuje serwer HTTP")
	interval := flag.Duration("interval", time.Second, "co ile zbierać próbkę metryk")
	flag.Parse()

	kolektor := agent.NewCollector()
	// 2 godziny historii przy próbce co sekundę = 7200 miejsc w buforze.
	historia := agent.NewHistory(2*time.Hour, *interval)

	// `go` uruchamia pętlę zbierania w tle (goroutine) —
	// serwer HTTP działa równolegle i tylko czyta wyniki.
	go kolektor.Run(historia, *interval)

	srv := server.New(kolektor, historia, version)
	log.Printf("malleus %s — API pod http://localhost%s/api/v1/", version, *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
