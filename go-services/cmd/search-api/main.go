package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/JOMS-inc/JOMSearch/go-services/internal/api"
    "github.com/JOMS-inc/JOMSearch/go-services/internal/db"
)

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}

func main() {
	dbPath := "../db/whoknows.db"

    conn, err := db.Open(dbPath)
    if err != nil {
        log.Fatalf("failed to open db: %v", err)
    }

    defer conn.Close()

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request){
        w.Write([]byte("ok"))
    })

    api.RegisterRoutes(mux)
    api.RegisterSearchRoutes(mux)

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
