// MalleusOS — punkt startowy binarki `malleus`.
// Na razie szkielet: parsuje flagi i wystawia tymczasową stronę.
// W kolejnych krokach dojdą: agent metryk, API i strumień SSE.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Wersja wpisana na sztywno; przy wydaniach będzie podmieniana.
const version = "0.1.0-dev"

func main() {
	// flag.String definiuje opcję linii poleceń: ./malleus -addr :8443
	addr := flag.String("addr", ":8443", "adres, na którym nasłuchuje serwer HTTP")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "MalleusOS %s — panel w budowie\n", version)
	})

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
		// Limit czasu na nagłówki chroni przed wiszącymi połączeniami.
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("malleus %s nasłuchuje na %s", version, *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
