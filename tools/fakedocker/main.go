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
	"io"
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
		c := find(r.PathValue("id"))
		if c == nil {
			http.NotFound(w, r)
			return
		}
		// Tty=false → malleus musi rozplatać ramki, jak przy
		// prawdziwym Dockerze. Image/Env/HostConfig — dla aktualizacji.
		json.NewEncoder(w).Encode(map[string]any{
			"Image":  "sha256:" + c.Image,
			"Config": map[string]any{"Tty": false, "Env": []string{"TZ=Europe/Warsaw"}, "Labels": map[string]string{}},
			"HostConfig": map[string]any{
				"Binds": []string{}, "NetworkMode": "bridge",
				"PortBindings": map[string]any{},
			},
		})
	})

	mux.HandleFunc("GET /images/{name}/json", func(w http.ResponseWriter, r *http.Request) {
		// ID pochodzi od nazwy obrazu — zgadza się z kontenerem,
		// więc aktualizacja odpowie "masz już najnowszą wersję".
		json.NewEncoder(w).Encode(map[string]string{
			"Id": "sha256:" + r.PathValue("name"),
		})
	})

	mux.HandleFunc("GET /volumes/{name}", func(w http.ResponseWriter, r *http.Request) {
		// Udawany wolumen: prawdziwy katalog z plikiem, żeby kopia
		// zapasowa miała co pakować.
		dir := "/tmp/fakevols/" + r.PathValue("name")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(dir+"/przykladowe-dane.txt", []byte("dane aplikacji\n"), 0o644)
		json.NewEncoder(w).Encode(map[string]string{"Mountpoint": dir})
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

	mux.HandleFunc("GET /containers/{id}/stats", func(w http.ResponseWriter, r *http.Request) {
		if find(r.PathValue("id")) == nil {
			http.NotFound(w, r)
			return
		}
		// Udawane, ale wiarygodne liczby: precpu/cpu z roznica.
		fmt.Fprint(w, `{"cpu_stats":{"cpu_usage":{"total_usage":2000000000},"system_cpu_usage":100000000000,"online_cpus":4},
			"precpu_stats":{"cpu_usage":{"total_usage":1990000000},"system_cpu_usage":99000000000},
			"memory_stats":{"usage":268435456}}`)
	})

	// --- budowanie obrazow (Moje projekty) ---

	mux.HandleFunc("POST /build", func(w http.ResponseWriter, r *http.Request) {
		// Prawdziwy Docker czyta kontekst tar — my go tylko połykamy.
		io.Copy(io.Discard, r.Body)
		fl := w.(http.Flusher)
		steps := []string{
			"Step 1/3 : FROM alpine:latest",
			"Step 2/3 : COPY . /app",
			"Step 3/3 : CMD [\"/app/start\"]",
			"Successfully built abcdef123456",
			"Successfully tagged " + r.URL.Query().Get("t"),
		}
		for _, s := range steps {
			fmt.Fprintf(w, `{"stream":%q}`+"\n", s+"\n")
			fl.Flush()
			time.Sleep(300 * time.Millisecond)
		}
	})

	mux.HandleFunc("DELETE /images/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[{"Deleted":"ok"}]`)
	})

	// --- endpointy instalacji (katalog aplikacji) ---

	mux.HandleFunc("POST /images/create", func(w http.ResponseWriter, r *http.Request) {
		// Udawany postęp pobierania obrazu — linie JSON z licznikami
		// bajtów per warstwa, jak z prawdziwego rejestru. Dwie
		// warstwy pobierane "równolegle", potem rozpakowywanie.
		fl := w.(http.Flusher)
		line := func(status, id string, cur, tot int) {
			fmt.Fprintf(w,
				`{"status":%q,"id":%q,"progressDetail":{"current":%d,"total":%d}}`+"\n",
				status, id, cur, tot)
			fl.Flush()
		}
		fmt.Fprintln(w, `{"status":"Pulling fs layer","id":"aa11"}`)
		fl.Flush()
		for i := 1; i <= 4; i++ {
			line("Downloading", "aa11", i*250, 1000)
			line("Downloading", "bb22", i*500, 2000)
			time.Sleep(200 * time.Millisecond)
		}
		for i := 1; i <= 2; i++ {
			line("Extracting", "aa11", i*500, 1000)
			line("Extracting", "bb22", i*1000, 2000)
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Fprintln(w, `{"status":"Pull complete","id":"bb22"}`)
		fl.Flush()
	})

	mux.HandleFunc("POST /containers/create", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		var body struct {
			Image string `json:"Image"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		containers = append(containers, &fakeContainer{
			ID: "fake" + name + "00000000", Name: name,
			Image: body.Image, State: "exited",
		})
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"Id":"fake%s00000000"}`, name)
	})

	mux.HandleFunc("DELETE /containers/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		for i, c := range containers {
			if c.ID == id || c.Name == id {
				containers = append(containers[:i], containers[i+1:]...)
				break
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

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
