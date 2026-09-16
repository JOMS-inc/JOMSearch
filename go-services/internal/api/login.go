package api

import (
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var loginTmpl = templates.Page("login.html")

type loginPageData struct {
	templates.BaseData
	Error    string
	Username string
}

// LoginHandler servér login-siden (GET /login).
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