package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BuildImage buduje obraz z kontekstu (archiwum tar z Dockerfile
// w środku) i nadaje mu tag. Docker strumieniuje postęp jako linie
// JSON — każdą sensowną przekazujemy przez onLine, dzięki czemu
// użytkownik widzi budowanie NA ŻYWO. Bez limitu czasu (builds
// bywają długie) — przerwanie kontroluje ctx.
func (c *Client) BuildImage(ctx context.Context, tag string, contextTar io.Reader, onLine func(string)) error {
	u := "http://docker/build?t=" + url.QueryEscape(tag) + "&dockerfile=Dockerfile&rm=1"
	req, err := http.NewRequestWithContext(ctx, "POST", u, contextTar)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-tar")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("budowanie: %s (%s)", res.Status, strings.TrimSpace(string(body)))
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var line struct {
			Stream string `json:"stream"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Error != "" {
			return fmt.Errorf("budowanie: %s", line.Error)
		}
		if txt := strings.TrimRight(line.Stream, "\n"); txt != "" {
			onLine(txt)
		} else if line.Status != "" {
			onLine(line.Status)
		}
	}
	return sc.Err()
}

// RemoveImage usuwa obraz (np. przy kasowaniu projektu).
func (c *Client) RemoveImage(ctx context.Context, tag string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		"http://docker/images/"+url.PathEscape(tag)+"?force=1", nil)
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
		return fmt.Errorf("usuwanie obrazu: %s (%s)", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
