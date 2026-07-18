// Pakiet docker rozmawia z demonem Dockera przez jego socket
// uniksowy — zwykłym HTTP ze stdlib, bez oficjalnego SDK
// (które ciągnie dziesiątki zależności).
//
// Socket uniksowy to "gniazdko" w systemie plików
// (/var/run/docker.sock), przez które Docker wystawia REST API —
// dokładnie takie samo jak po sieci, tylko lokalne.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client to cienki klient API Dockera.
type Client struct {
	http *http.Client
}

// New tworzy klienta gadającego przez podany socket.
func New(socketPath string) *Client {
	return &Client{
		http: &http.Client{
			Transport: &http.Transport{
				// Zamiast łączyć się po TCP z hostem z adresu URL,
				// zawsze wbijamy się w socket uniksowy.
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
			// Brak globalnego limitu czasu — logi płyną godzinami.
			// Krótkie wywołania dostają limit per żądanie (kontekst).
		},
	}
}

// Container to jeden kontener w formie, którą wysyłamy do panelu.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`  // running / exited / paused…
	Status string `json:"status"` // np. "Up 2 hours"
}

// Ping sprawdza, czy demon w ogóle odpowiada.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://docker/_ping", nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

// List zwraca wszystkie kontenery (też zatrzymane).
func (c *Client) List(ctx context.Context) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Surowa odpowiedź Dockera — bierzemy tylko potrzebne pola.
	var raw []struct {
		ID     string   `json:"Id"`
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	out := make([]Container, 0, len(raw))
	for _, r := range raw {
		name := r.ID[:12]
		if len(r.Names) > 0 {
			// Docker poprzedza nazwy ukośnikiem: "/jellyfin"
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		out = append(out, Container{
			ID: r.ID, Name: name, Image: r.Image,
			State: r.State, Status: r.Status,
		})
	}
	return out, nil
}

// Action wykonuje start / stop / restart na kontenerze.
func (c *Client) Action(ctx context.Context, id, action string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("nieznana akcja %q", action)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://docker/containers/"+id+"/"+action, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 204 = zrobione, 304 = już był w tym stanie — oba są OK.
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("docker: %s (%s)", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// isTTY sprawdza, czy kontener ma terminal — od tego zależy
// format strumienia logów (patrz logs.go).
func (c *Client) isTTY(ctx context.Context, id string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/containers/"+id+"/json", nil)
	if err != nil {
		return false, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	var info struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return false, err
	}
	return info.Config.Tty, nil
}
