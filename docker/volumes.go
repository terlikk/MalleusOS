package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// VolumeMountpoint pyta Dockera, gdzie na dysku serwera leży
// nazwany wolumen (typowo /var/lib/docker/volumes/<nazwa>/_data).
// Potrzebne do kopii zapasowych — pakujemy ten katalog do tar.gz.
func (c *Client) VolumeMountpoint(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/volumes/"+url.PathEscape(name), nil)
	if err != nil {
		return "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("wolumen %s: %s", name, res.Status)
	}

	var info struct {
		Mountpoint string `json:"Mountpoint"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Mountpoint, nil
}
