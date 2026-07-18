package storage

// Projekty użytkownika ("Moje projekty") — własne aplikacje
// wdrożone z repozytorium git albo pliku ZIP.

// Project opisuje jeden wdrożony projekt.
type Project struct {
	Name          string `json:"name"`          // też subdomena: nazwa.malleus.local
	GitURL        string `json:"gitUrl"`        // puste przy wdrożeniu z ZIP-a
	ContainerPort int    `json:"containerPort"` // port aplikacji WEWNĄTRZ kontenera
	HostPort      int    `json:"hostPort"`      // port wystawiony na serwerze
	Created       int64  `json:"created"`       // unix ms
}

func (d *DB) SaveProject(p Project) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO projects
		(name, git_url, container_port, host_port, created)
		VALUES (?, ?, ?, ?, ?)`,
		p.Name, p.GitURL, p.ContainerPort, p.HostPort, p.Created)
	return err
}

func (d *DB) ListProjects() ([]Project, error) {
	rows, err := d.db.Query(`SELECT name, git_url, container_port, host_port, created
		FROM projects ORDER BY created`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Name, &p.GitURL, &p.ContainerPort, &p.HostPort, &p.Created); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) DeleteProject(name string) error {
	_, err := d.db.Exec(`DELETE FROM projects WHERE name = ?`, name)
	return err
}
