package api

import (
	"database/sql"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /{$}", Root(db))
	mux.HandleFunc("GET /register", RegisterHandler)
	mux.HandleFunc("GET /login", LoginHandler)
	mux.HandleFunc("GET /weather", WeatherPage)
}