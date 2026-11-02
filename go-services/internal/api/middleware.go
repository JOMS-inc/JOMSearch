package api

import (
	"net/http"
	"strings"
)

// RequirePasswordChange locks flagged users (see db.Migrate) to the
// change-password page. It wraps the whole mux, so routes added later are
// covered automatically. Because currentUser reads the flag from the DB on
// every request, sessions created before the breach are locked too.
func RequirePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !passwordChangeExempt(r.URL.Path) {
			if user := currentUser(r); user != nil && user.MustChangePassword {
				http.Redirect(w, r, "/change-password", http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func passwordChangeExempt(path string) bool {
	switch path {
	case "/change-password", "/logout", "/health":
		return true
	}
	return strings.HasPrefix(path, "/static/")
}
