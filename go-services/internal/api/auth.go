package api

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/sessions"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var store = newCookieStore()

// dbConn is set once at startup via SetDB so currentUser can look up the
// full user record (id, username, email) behind a session.
var dbConn *sql.DB

// SetDB must be called once at startup, before any request comes in.
func SetDB(db *sql.DB) {
	dbConn = db
}

func newCookieStore() *sessions.CookieStore {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		// Fine for local dev; production must set SESSION_SECRET or every
		// session cookie is signed with a key anyone can read in git history.
		log.Println("WARNING: SESSION_SECRET not set, using an insecure default key")
		secret = "dev-only-insecure-key-change-me"
	}
	return sessions.NewCookieStore([]byte(secret))
}

// currentUser resolves the logged-in user (if any) from the session cookie,
// looking up the full record (including Username) from the database.
func currentUser(r *http.Request) *templates.User {
	sess, err := store.Get(r, "session")
	if err != nil {
		return nil
	}

	email, ok := sess.Values["email"].(string)
	if !ok || email == "" {
		return nil
	}

	if dbConn == nil {
		return nil
	}

	var u templates.User
	err = dbConn.QueryRow(`SELECT id, username, email FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Username, &u.Email)
	if err != nil {
		return nil
	}
	return &u
}

// loginSession marks the request's session as authenticated for email and
// saves the cookie onto the response.
func loginSession(w http.ResponseWriter, r *http.Request, email string) error {
	sess, err := store.Get(r, "session")
	if err != nil {
		// store.Get returns a usable new session even on error (e.g. a
		// tampered/undecodable existing cookie); proceed with it.
		if sess == nil {
			return err
		}
	}

	sess.Values["email"] = email
	return sess.Save(r, w)
}

// logoutSession clears the current session's auth state.
func logoutSession(w http.ResponseWriter, r *http.Request) error {
	sess, err := store.Get(r, "session")
	if err != nil {
		if sess == nil {
			return err
		}
	}

	delete(sess.Values, "email")
	sess.Options.MaxAge = -1 // expire the cookie immediately
	return sess.Save(r, w)
}

