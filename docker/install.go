package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PullImage pobiera obraz z rejestru. Docker strumieniuje postęp
// jako linie JSON — czytamy je do końca i wyłapujemy błędy
// (np. "obraz nie istnieje"). Bez limitu czasu: duże obrazy na
// wolnym łączu potrafią schodzić długo, o przerwaniu decyduje ctx.
func (c *Client) PullImage(ctx context.Context, image string) error {
	u := "http://docker/images/create?fromImage=" + url.QueryEscape(image)
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("pobieranie obrazu: %s (%s)", res.Status, strings.TrimSpace(string(body)))
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		var line struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.Error != "" {
			return fmt.Errorf("pobieranie obrazu: %s", line.Error)
		}
	}
	return sc.Err()
}

// CreateSpec opisuje kontener do utworzenia.
type CreateSpec struct {
	Image   string
	Env     []string          // "KLUCZ=wartość"
	Cmd     []string          // polecenie startowe (puste = domyślne obrazu)
	Ports   []PortMap         // mapowania host→kontener
	Binds   []string          // "wolumen:/ścieżka"
	Labels  map[string]string // etykiety, np. malleus.app=jellyfin
	Network string            // "host" = sieć współdzielona z serwerem
}

type PortMap struct {
	Host      int
	Container int
	Protocol  string // puste = tcp
}

// CreateContainer tworzy (ale nie uruchamia) kontener o podanej
// nazwie. Restart policy "unless-stopped": po restarcie serwera
// aplikacje wstają same, chyba że ktoś je celowo zatrzymał.
func (c *Client) CreateContainer(ctx context.Context, name string, spec CreateSpec) error {
	exposed := map[string]struct{}{}
	bindings := map[string][]map[string]string{}
	for _, p := range spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		key := fmt.Sprintf("%d/%s", p.Container, proto)
		exposed[key] = struct{}{}
		bindings[key] = append(bindings[key], map[string]string{
			"HostPort": fmt.Sprintf("%d", p.Host),
		})
	}

	hostConfig := map[string]any{
		"PortBindings":  bindings,
		"Binds":         spec.Binds,
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
	}
	if spec.Network != "" {
		// W trybie "host" kontener używa sieci serwera wprost —
		// mapowania portów nie mają wtedy zastosowania.
		hostConfig["NetworkMode"] = spec.Network
	}

	body := map[string]any{
		"Image":        spec.Image,
		"Env":          spec.Env,
		"Labels":       spec.Labels,
		"ExposedPorts": exposed,
		"HostConfig":   hostConfig,
	}
	if len(spec.Cmd) > 0 {
		body["Cmd"] = spec.Cmd
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://docker/containers/create?name="+url.QueryEscape(name),
		bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("tworzenie kontenera: %s (%s)", res.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

// RemoveContainer usuwa kontener (force = także działający).
// Wolumenów z danymi celowo NIE kasujemy — odinstalowanie
// aplikacji nie powinno kasować czyichś zdjęć.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		"http://docker/containers/"+url.PathEscape(id)+"?force=1", nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("usuwanie kontenera: %s (%s)", res.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}
