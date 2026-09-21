package api

import (
	"database/sql"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var registerTmpl = templates.Page("register.html")

type registerPageData struct {
	templates.BaseData
	Error    string
	Username string
	Email    string
}

// RegisterHandler serves the registration page (GET /register).
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if user := currentUser(r); user != nil {
		http.Redirect(w, r, "/search", http.StatusSeeOther)
		return
	}

	data := registerPageData{}

	if err := registerTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// RegisterSubmitHandler handles the registration form submission
// (POST /api/register): validates input, hashes the password, creates
// the user, starts a session, and redirects to /search.
func RegisterSubmitHandler(db *sql.DB) http.HandlerFunc {
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
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		password2 := r.FormValue("password2")

		renderErr := func(msg string, status int) {
			data := registerPageData{
				Error:    msg,
				Username: username,
				Email:    email,
			}
			w.WriteHeader(status)
			registerTmpl.ExecuteTemplate(w, "layout.html", data)
		}

		if username == "" || email == "" || password == "" {
			renderErr("All fields are required.", http.StatusUnprocessableEntity)
			return
		}
		if password != password2 {
			renderErr("Passwords do not match.", http.StatusUnprocessableEntity)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec(
			`INSERT INTO users (username, email, password) VALUES (?, ?, ?)`,
			username, email, string(hash),
		)
		if err != nil {
			// Adjust this check to match your driver's actual unique-constraint error.
			if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
				renderErr("Username or email already taken.", http.StatusConflict)
				return
			}
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		if err := loginSession(w, r, email); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/search", http.StatusSeeOther)
	}
}
