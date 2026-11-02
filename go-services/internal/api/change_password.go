package api

import (
	"database/sql"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

const minPasswordLength = 8

var changePasswordTmpl = templates.Page("change_password.html")

type changePasswordPageData struct {
	templates.BaseData
	Error string
}

func renderChangePassword(w http.ResponseWriter, user *templates.User, msg string, status int) {
	data := changePasswordPageData{BaseData: templates.BaseData{User: user}, Error: msg}
	w.WriteHeader(status)
	if err := changePasswordTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		log.Printf("change-password: render page: %v", err)
	}
}

// ChangePasswordHandler serves the change-password page (GET /change-password).
func ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	renderChangePassword(w, user, "", http.StatusOK)
}

// ChangePasswordSubmitHandler handles POST /change-password: verifies the
// current password, stores a bcrypt hash of the new one and clears the
// must_change_password flag.
func ChangePasswordSubmitHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if err := parseInput(r); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		current := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		newPassword2 := r.FormValue("new_password2")

		fail := func(msg string) {
			renderChangePassword(w, user, msg, http.StatusUnprocessableEntity)
		}

		var stored string
		if err := db.QueryRow(`SELECT password FROM users WHERE id = ?`, user.ID).Scan(&stored); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		switch {
		case !checkPassword(current, stored):
			fail("Current password is wrong.")
			return
		case len(newPassword) < minPasswordLength:
			fail("New password must be at least 8 characters.")
			return
		case newPassword != newPassword2:
			fail("New passwords do not match.")
			return
		case newPassword == current:
			fail("New password must be different from the old one.")
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if _, err := db.Exec(
			`UPDATE users SET password = ?, must_change_password = 0 WHERE id = ?`,
			string(hash), user.ID,
		); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/search", http.StatusSeeOther)
	}
}
