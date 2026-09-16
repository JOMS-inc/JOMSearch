package main

import (
	"log"
	"net/http"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/api"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
)

func main() {
	dbPath := "../db/whoknows.db"

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

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}