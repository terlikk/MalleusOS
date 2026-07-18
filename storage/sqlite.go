// Pakiet storage trzyma historię próbek w SQLite.
//
// Używamy modernc.org/sqlite — sterownika napisanego w czystym Go
// (bez CGO), więc kompilacja krzyżowa amd64+arm64 dalej działa
// jednym poleceniem. Baza to jeden plik w katalogu data/.
package storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	// Pusty import rejestruje sterownik "sqlite" w database/sql.
	_ "modernc.org/sqlite"

	"malleus/agent"
)

// DB opakowuje połączenie z bazą. Spełnia interfejs agent.Store,
// więc dla kolektora i serwera jest wymienne z historią w pamięci.
type DB struct {
	db *sql.DB
}

// Open otwiera (lub zakłada) bazę pod podaną ścieżką.
func Open(path string) (*DB, error) {
	// Katalog bazy może jeszcze nie istnieć — tworzymy go.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// WAL (Write-Ahead Log): zapisy idą do osobnego dziennika,
	// dzięki czemu odczyty nie blokują się z zapisami.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, err
	}

	// Tabele: samples (czas próbki + pełny JSON — czas jest kluczem
	// głównym, więc zapytania po zakresie są szybkie), config
	// (ustawienia typu klucz→wartość, m.in. hash hasła) i sessions
	// (tokeny zalogowanych — przeżywają restart serwera).
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS samples (
			time INTEGER PRIMARY KEY,
			data TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token   TEXT PRIMARY KEY,
			expires INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS projects (
			name           TEXT PRIMARY KEY,
			git_url        TEXT NOT NULL DEFAULT '',
			container_port INTEGER NOT NULL,
			host_port      INTEGER NOT NULL,
			created        INTEGER NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

// Add zapisuje próbkę. Błąd tylko logujemy — pojedyncza zgubiona
// próbka nie jest powodem, żeby zatrzymywać cały system.
func (d *DB) Add(s agent.Sample) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	if _, err := d.db.Exec(
		`INSERT OR REPLACE INTO samples (time, data) VALUES (?, ?)`,
		s.Time, string(data),
	); err != nil {
		log.Printf("storage: zapis próbki nieudany: %v", err)
	}
}

// Since zwraca próbki z ostatniego okresu, przerzedzone do
// maxPoints — dokładnie jak historia w pamięci.
func (d *DB) Since(dur time.Duration, maxPoints int) []agent.Sample {
	cutoff := time.Now().Add(-dur).UnixMilli()
	rows, err := d.db.Query(
		`SELECT data FROM samples WHERE time >= ? ORDER BY time`, cutoff)
	if err != nil {
		log.Printf("storage: odczyt historii nieudany: %v", err)
		return nil
	}
	defer rows.Close()

	var out []agent.Sample
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var s agent.Sample
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return agent.Downsample(out, maxPoints)
}

// PruneLoop co `every` kasuje próbki starsze niż `keep`,
// żeby plik bazy nie rósł w nieskończoność.
// Uruchamiaj w goroutine: go db.PruneLoop(24*time.Hour, time.Hour)
func (d *DB) PruneLoop(keep, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-keep).UnixMilli()
		if _, err := d.db.Exec(`DELETE FROM samples WHERE time < ?`, cutoff); err != nil {
			log.Printf("storage: sprzątanie nieudane: %v", err)
		}
	}
}
