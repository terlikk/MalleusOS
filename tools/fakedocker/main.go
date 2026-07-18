// fakedocker — atrapa API Dockera do developmentu bez Dockera.
//
// Wystawia na sockecie uniksowym minimalny wycinek prawdziwego
// API: listę kontenerów, start/stop/restart i logi płynące co
// ~700 ms w ORYGINALNYM formacie multipleksowanym (8-bajtowe
// nagłówki ramek) — dzięki temu ćwiczymy dokładnie tę samą
// ścieżkę kodu, co przy prawdziwym Dockerze.
//
// Użycie:
//
//	go run ./tools/fakedocker            # socket: /tmp/fakedocker.sock
//	./bin/malleus -docker-sock /tmp/fakedocker.sock
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// fakeContainer trzyma stan jednego udawanego kontenera.
type fakeContainer struct {
	ID     string
	Name   string
	Image  string
	State  string // "running" albo "exited"
}

var (
	mu         sync.Mutex
	containers = []*fakeContainer{
		{ID: "aaaa1111aaaa", Name: "jellyfin", Image: "jellyfin/jellyfin:latest", State: "running"},
		{ID: "bbbb2222bbbb", Name: "nextcloud", Image: "nextcloud:29", State: "running"},
		{ID: "cccc3333cccc", Name: "pi-hole", Image: "pihole/pihole:latest", State: "exited"},
	}
)

func find(id string) *fakeContainer {
	mu.Lock()
	defer mu.Unlock()
	for _, c := range containers {
		if c.ID == id || c.Name == id {
			return c
		}
	}
	return nil
}

func main() {
	sock := flag.String("sock", "/tmp/fakedocker.sock", "ścieżka socketu")
	flag.Parse()

	// Stary socket po poprzednim uruchomieniu trzeba sprzątnąć,
	// inaczej Listen powie "address already in use".
	os.Remove(*sock)
	ln, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fakedocker nasłuchuje na %s", *sock)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /_ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		type item struct {
			ID     string   `json:"Id"`
			Names  []string `json:"Names"`
			Image  string   `json:"Image"`
			State  string   `json:"State"`
			Status string   `json:"Status"`
		}
		var out []item
		for _, c := range containers {
			status := "Exited (0) 5 minutes ago"
			if c.State == "running" {
				status = "Up 42 minutes"
			}
			out = append(out, item{
				ID: c.ID, Names: []string{"/" + c.Name},
				Image: c.Image, State: c.State, Status: status,
			})
		}
		json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("GET /containers/{id}/json", func(w http.ResponseWriter, r *http.Request) {
		if find(r.PathValue("id")) == nil {
			http.NotFound(w, r)
			return
		}
		// Tty=false → malleus musi rozplatać ramki, jak przy
		// prawdziwym Dockerze.
		fmt.Fprint(w, `{"Config":{"Tty":false}}`)
	})

	for _, action := range []string{"start", "stop", "restart"} {
		newState := map[string]string{"start": "running", "stop": "exited", "restart": "running"}[action]
		mux.HandleFunc("POST /containers/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
			c := find(r.PathValue("id"))
			if c == nil {
				http.NotFound(w, r)
				return
			}
			mu.Lock()
			c.State = newState
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		})
	}

	mux.HandleFunc("GET /containers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		c := find(r.PathValue("id"))
		if c == nil {
			http.NotFound(w, r)
			return
		}
		fl := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")

		// Ramka multipleksowana: [typ 0 0 0 dł dł dł dł] + tekst
		frame := func(stream byte, line string) {
			payload := []byte(line + "\n")
			header := make([]byte, 8)
			header[0] = stream
			binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
			w.Write(header)
			w.Write(payload)
			fl.Flush()
		}

		for i := 1; ; i++ {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(700 * time.Millisecond):
				if i%5 == 0 {
					frame(2, fmt.Sprintf("[%s] WARN  coś na stderr, linia %d", c.Name, i))
				} else {
					frame(1, fmt.Sprintf("[%s] INFO  udawany log, linia %d", c.Name, i))
				}
			}
		}
	})

	log.Fatal(http.Serve(ln, mux))
}
