package api

import (
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var aboutTmpl = templates.Page("about.html")

// AboutHandler serves the about page (GET /about).
func AboutHandler(w http.ResponseWriter, r *http.Request) {
	data := templates.BaseData{User: currentUser(r)}

	if err := aboutTmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
