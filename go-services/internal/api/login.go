package api

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var loginTmpl = templates.Page("login.html")

type loginPageData struct {
	templates.BaseData
	Error    string
	Username string
}

// LoginHandler serves the login page (GET /login).
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if user := currentUser(r); user != nil {
		http.Redirect(w, r, "/search", http.StatusSeeOther)
		return
	}

	data := loginPageData{}

	if err := loginTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// LoginSubmitHandler handles the login form submission (POST /login):
// verifies credentials, starts a session, and redirects to /search.
func LoginSubmitHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user := currentUser(r); user != nil {
			http.Redirect(w, r, "/search", http.StatusSeeOther)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		renderErr := func(msg string) {
			data := loginPageData{Error: msg, Username: username}
			w.WriteHeader(http.StatusUnauthorized)
			loginTmpl.ExecuteTemplate(w, "layout.html", data)
		}

		if username == "" || password == "" {
			renderErr("Username and password are required.")
			return
		}

		var id int64
		var email string
		var stored string
		err := db.QueryRow(`SELECT id, email, password FROM users WHERE username = ?`, username).
			Scan(&id, &email, &stored)
		if err == sql.ErrNoRows {
			renderErr("Invalid username or password.")
			return
		} else if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		if !checkPassword(password, stored) {
			renderErr("Invalid username or password.")
			return
		}

		// Legacy MD5 rows (e.g. the seeded admin user) get silently upgraded
		// to bcrypt the first time they successfully log in.
		if looksLikeMD5(stored) {
			if newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err == nil {
				db.Exec(`UPDATE users SET password = ? WHERE id = ?`, string(newHash), id)
			}
		}

		if err := loginSession(w, r, email); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/search", http.StatusSeeOther)
	}
}

// looksLikeMD5 reports whether stored is a bare 32-char hex string, the
// format used by the old seed data (e.g. the 'admin' user's password).
func looksLikeMD5(stored string) bool {
	if len(stored) != 32 {
		return false
	}
	_, err := hex.DecodeString(stored)
	return err == nil
}

// checkPassword verifies password against stored, which may be either a
// bcrypt hash (new rows) or a legacy bare MD5 hex digest (old seed data).
//
// SECURITY NOTE: MD5 is cryptographically broken and must never be used for
// new password storage — bcrypt.GenerateFromPassword (see register.go) is
// the only path that creates new rows, so this branch only ever runs
// against pre-existing legacy data (e.g. the seeded 'admin' row) that
// predates this codebase using bcrypt. LoginSubmitHandler rewrites the row
// to a bcrypt hash immediately after a successful legacy match, so this
// branch is self-eliminating: once every row has logged in once, it is
// dead code and should be deleted.
func checkPassword(password, stored string) bool {
	if looksLikeMD5(stored) {
		sum := md5.Sum([]byte(password)) // #nosec G401 -- legacy hash verification only, see note above; never used to create new hashes
		return hex.EncodeToString(sum[:]) == stored
	}
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
}
