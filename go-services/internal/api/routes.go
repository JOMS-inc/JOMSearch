package api

import (
	"database/sql"
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/static"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static.FS)))

	mux.HandleFunc("GET /{$}", Root(db))

	mux.HandleFunc("GET /search", Root(db))

	mux.HandleFunc("GET /register", RegisterHandler)
	mux.HandleFunc("POST /api/register", RegisterSubmitHandler(db))

	mux.HandleFunc("GET /login", LoginHandler)
	mux.HandleFunc("POST /login", LoginSubmitHandler(db))
	mux.HandleFunc("POST /api/login", LoginSubmitHandler(db))

	mux.HandleFunc("GET /logout", LogoutHandler)

	mux.HandleFunc("GET /change-password", ChangePasswordHandler)
	mux.HandleFunc("POST /change-password", ChangePasswordSubmitHandler(db))

	mux.HandleFunc("GET /weather", WeatherPage)

	mux.HandleFunc("GET /about", AboutHandler)
}

// LogoutHandler clears the current session and redirects to /login.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := logoutSession(w, r); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
