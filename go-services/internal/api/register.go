package api

import (
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var registerTmpl = templates.Page("register.html")

type registerPageData struct {
	templates.BaseData
	Error    string
	Username string
	Email    string
}

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