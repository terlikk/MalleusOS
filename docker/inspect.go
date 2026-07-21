package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ContainerInfo to konfiguracja istniejącego kontenera — tyle,
// ile trzeba, żeby odtworzyć go 1:1 na nowszym obrazie.
type ContainerInfo struct {
	ImageID     string
	Env         []string
	Labels      map[string]string
	Binds       []string
	NetworkMode string
	Ports       []PortMap
}

// InspectContainer czyta pełną konfigurację kontenera.
func (c *Client) InspectContainer(ctx context.Context, id string) (ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/containers/"+url.PathEscape(id)+"/json", nil)
	if err != nil {
		return ContainerInfo{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return ContainerInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return ContainerInfo{}, fmt.Errorf("kontener %s: %s", id, res.Status)
	}

	var raw struct {
		Image  string `json:"Image"`
		Config struct {
			Env    []string          `json:"Env"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		HostConfig struct {
			Binds        []string                       `json:"Binds"`
			NetworkMode  string                         `json:"NetworkMode"`
			PortBindings map[string][]map[string]string `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return ContainerInfo{}, err
	}

	info := ContainerInfo{
		ImageID: raw.Image,
		Env:     raw.Config.Env,
		Labels:  raw.Config.Labels,
		Binds:   raw.HostConfig.Binds,
	}
	if raw.HostConfig.NetworkMode == "host" {
		info.NetworkMode = "host"
	}
	// PortBindings: klucz "8096/tcp" → PortMap{Container, Protocol, Host}
	for key, bindings := range raw.HostConfig.PortBindings {
		portStr, proto, _ := strings.Cut(key, "/")
		containerPort, _ := strconv.Atoi(portStr)
		for _, b := range bindings {
			hostPort, _ := strconv.Atoi(b["HostPort"])
			info.Ports = append(info.Ports, PortMap{
				Host: hostPort, Container: containerPort, Protocol: proto,
			})
		}
	}
	return info, nil
}

// ImageID zwraca identyfikator (sha256) obrazu o podanej nazwie —
// po ściągnięciu nowej wersji ID się zmienia, a to nasz sygnał,
// że jest co aktualizować.
func (c *Client) ImageID(ctx context.Context, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/images/"+url.PathEscape(ref)+"/json", nil)
	if err != nil {
		return "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("obraz %s: %s", ref, res.Status)
	}
	var info struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.ID, nil
}
