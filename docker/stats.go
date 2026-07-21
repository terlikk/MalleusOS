package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ContainerStats to chwilowe zużycie zasobów jednego kontenera.
type ContainerStats struct {
	CPUPercent float64 `json:"cpuPercent"`
	MemBytes   uint64  `json:"memBytes"`
}

// Stats pyta Dockera o zużycie kontenera. stream=false: Docker
// robi dwa pomiary w ~sekundę i zwraca jeden wynik — z różnicy
// liczników liczymy procent CPU (ten sam trik co w naszym agencie).
func (c *Client) Stats(ctx context.Context, id string) (ContainerStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://docker/containers/"+url.PathEscape(id)+"/stats?stream=false", nil)
	if err != nil {
		return ContainerStats{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return ContainerStats{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return ContainerStats{}, fmt.Errorf("stats: %s", res.Status)
	}

	var raw struct {
		CPU struct {
			Usage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
			Online uint64 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPU struct {
			Usage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		Memory struct {
			Usage uint64 `json:"usage"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return ContainerStats{}, err
	}

	out := ContainerStats{MemBytes: raw.Memory.Usage}
	cpuDelta := float64(raw.CPU.Usage.Total) - float64(raw.PreCPU.Usage.Total)
	sysDelta := float64(raw.CPU.System) - float64(raw.PreCPU.System)
	if cpuDelta > 0 && sysDelta > 0 {
		cpus := float64(raw.CPU.Online)
		if cpus == 0 {
			cpus = 1
		}
		out.CPUPercent = cpuDelta / sysDelta * cpus * 100
	}
	return out, nil
}
