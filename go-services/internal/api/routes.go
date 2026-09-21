package api

import (
	"database/sql"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /{$}", Root(db))

	mux.HandleFunc("GET /register", RegisterHandler)
	mux.HandleFunc("POST /api/register", RegisterSubmitHandler(db))

	mux.HandleFunc("GET /login", LoginHandler)
	mux.HandleFunc("POST /login", LoginSubmitHandler(db))

	mux.HandleFunc("GET /logout", LogoutHandler)

	mux.HandleFunc("GET /weather", WeatherPage)
}

// LogoutHandler clears the current session and redirects to /login.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
