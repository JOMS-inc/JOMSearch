package main

import (
	"log"
	"net/http"
	"os"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/api"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
)

func main() {
	dbPath := getEnv("DB_PATH", "../db/whoknows.db")
	port := getEnv("PORT", "8080")

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer conn.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	api.RegisterRoutes(mux, conn)
	api.RegisterSearchRoutes(mux, conn)
	api.SetDB(conn) // required so currentUser() can look users up

	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// getEnv returns the value of the environment variable key, or fallback if it is unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
