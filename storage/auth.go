package storage

import "log"

// Metody magazynu logowania — spełniają interfejs server.AuthStore.

// PasswordHash zwraca hash bcrypt hasła administratora
// (pusty łańcuch, gdy hasła jeszcze nie ustawiono).
func (d *DB) PasswordHash() string {
	var hash string
	err := d.db.QueryRow(
		`SELECT value FROM config WHERE key = 'password_hash'`).Scan(&hash)
	if err != nil {
		return "" // brak wiersza = brak hasła; inne błędy też traktujemy jak brak
	}
	return hash
}

func (d *DB) SetPasswordHash(hash string) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO config (key, value) VALUES ('password_hash', ?)`, hash)
	return err
}

func (d *DB) CreateSession(token string, expires int64) error {
	_, err := d.db.Exec(
		`INSERT INTO sessions (token, expires) VALUES (?, ?)`, token, expires)
	return err
}

// SessionValid sprawdza, czy token istnieje i nie wygasł.
// Przy okazji leniwie sprząta przeterminowane sesje.
func (d *DB) SessionValid(token string, now int64) bool {
	if _, err := d.db.Exec(`DELETE FROM sessions WHERE expires < ?`, now); err != nil {
		log.Printf("storage: sprzątanie sesji: %v", err)
	}
	var expires int64
	err := d.db.QueryRow(
		`SELECT expires FROM sessions WHERE token = ?`, token).Scan(&expires)
	return err == nil && expires >= now
}

func (d *DB) DeleteSession(token string) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}
