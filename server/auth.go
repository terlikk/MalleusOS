package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AuthStore to magazyn danych logowania (implementuje go storage.DB).
// Hasło trzymamy jako hash bcrypt — funkcji celowo POWOLNEJ,
// dzięki czemu zgadywanie haseł na skradzionej bazie jest drogie.
type AuthStore interface {
	PasswordHash() string // "" gdy hasła jeszcze nie ustawiono
	SetPasswordHash(hash string) error
	CreateSession(token string, expires int64) error
	SessionValid(token string, now int64) bool
	DeleteSession(token string) error
}

const (
	sessionCookie = "malleus_session"
	sessionTTL    = 30 * 24 * time.Hour // sesja żyje 30 dni
)

// protect opakowuje handler wymogiem ważnej sesji.
// Gdy magazyn auth jest niedostępny (tryb bez bazy), przepuszcza
// wszystko — z ostrzeżeniem w logu przy starcie.
func (s *Server) protect(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			h(w, r)
			return
		}
		if s.auth.PasswordHash() == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": "najpierw ustaw hasło", "setup": true,
			})
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.auth.SessionValid(c.Value, time.Now().UnixMilli()) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wymagane logowanie"})
			return
		}
		h(w, r)
	}
}

// handleAuthStatus mówi panelowi, na jakim ekranie ma wystartować.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]bool{
			"authDisabled": true, "setupDone": true, "authenticated": true,
		})
		return
	}
	authed := false
	if c, err := r.Cookie(sessionCookie); err == nil {
		authed = s.auth.SessionValid(c.Value, time.Now().UnixMilli())
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"authDisabled":  false,
		"setupDone":     s.auth.PasswordHash() != "",
		"authenticated": authed,
	})
}

// handleSetup ustawia hasło PRZY PIERWSZYM uruchomieniu
// i od razu loguje. Drugi raz się nie da (409).
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "logowanie wyłączone (brak bazy)"})
		return
	}
	if s.auth.PasswordHash() != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "hasło już ustawione"})
		return
	}
	password, ok := readPassword(w, r)
	if !ok {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.auth.SetPasswordHash(string(hash)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.startSession(w)
}

// handleLogin sprawdza hasło i zakłada sesję.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || s.auth.PasswordHash() == "" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "najpierw ustaw hasło", "setup": true})
		return
	}
	password, ok := readPassword(w, r)
	if !ok {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(s.auth.PasswordHash()), []byte(password)) != nil {
		// Krótka pauza zniechęca do zgadywania hasła na pętli.
		time.Sleep(300 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "złe hasło"})
		return
	}
	s.startSession(w)
}

// handleLogout kasuje sesję po obu stronach: w bazie i w cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && s.auth != nil {
		_ = s.auth.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// startSession generuje losowy token, zapisuje go i ustawia cookie.
//
// HttpOnly = JavaScript nie ma dostępu do cookie (ochrona przed
// kradzieżą tokenu przez wstrzyknięty skrypt). SameSite=Lax =
// cookie nie poleci z żądaniami inicjowanymi z obcych stron.
func (s *Server) startSession(w http.ResponseWriter) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	token := hex.EncodeToString(raw)
	expires := time.Now().Add(sessionTTL)
	if err := s.auth.CreateSession(token, expires.UnixMilli()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		Expires: expires, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// readPassword wyciąga hasło z JSON-a i pilnuje minimalnej długości.
func readPassword(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "niepoprawny JSON"})
		return "", false
	}
	if len(body.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hasło musi mieć co najmniej 8 znaków"})
		return "", false
	}
	return body.Password, true
}
