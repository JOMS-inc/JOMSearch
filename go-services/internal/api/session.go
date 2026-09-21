package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

type session struct {
	userID  int64
	expires time.Time
}

var (
	sessionMu sync.Mutex
	sessions  = map[string]session{} // token -> session
	dbConn    *sql.DB
)

const sessionCookieName = "session_token"

// SetDB must be called once at startup so currentUser can look users up.
func SetDB(db *sql.DB) {
	dbConn = db
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createSession issues a new session for userID and sets the cookie on w.
func createSession(w http.ResponseWriter, userID int64) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}

	expires := time.Now().Add(24 * time.Hour)

	sessionMu.Lock()
	sessions[token] = session{userID: userID, expires: expires}
	sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
	return nil
}

// clearSession removes the session tied to the request's cookie, if any,
// and expires the cookie in the response.
func clearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		sessionMu.Lock()
		delete(sessions, c.Value)
		sessionMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// currentUser resolves the logged-in user (if any) from the session cookie.
// Returns nil if there is no cookie, an unknown/expired session, or no DB set.
func currentUser(r *http.Request) *templates.User {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}

	sessionMu.Lock()
	sess, ok := sessions[c.Value]
	sessionMu.Unlock()
	if !ok || time.Now().After(sess.expires) {
		return nil
	}

	if dbConn == nil {
		return nil
	}

	var u templates.User
	err = dbConn.QueryRow(`SELECT id, username, email FROM users WHERE id = ?`, sess.userID).
		Scan(&u.ID, &u.Username, &u.Email)
	if err != nil {
		return nil
	}
	return &u
}
