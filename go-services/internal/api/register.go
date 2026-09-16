// internal/api/register.go
package api

import (
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

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

	templates.Templates.ExecuteTemplate(w, "layout.html", data)
}
