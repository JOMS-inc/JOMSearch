package main

import (
	"log"
	"net/http"
	"os"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/api"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/db"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/metrics"
	"github.com/JOMS-inc/JOMSearch/go-services/internal/searchlog"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/jomsearch?sslmode=disable")
	port := getEnv("PORT", "8080")
	metricsPort := getEnv("METRICS_PORT", "2112")
	searchLogPath := getEnv("SEARCH_LOG", "../logs/searches.log")

	conn, err := db.Open(dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("failed to migrate db: %v", err)
	}

	searchLogger, logFile, err := searchlog.Open(searchLogPath)
	if err != nil {
		log.Fatalf("failed to open search log: %v", err)
	}
	defer logFile.Close() //nolint:errcheck // nothing to do if closing fails at shutdown
	api.SetSearchLogger(searchLogger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	api.RegisterRoutes(mux, conn)
	api.RegisterSearchRoutes(mux, conn)
	api.SetDB(conn) // required so currentUser() can look users up

	// /metrics gets its own port. The app port is public (nginx and :8080),
	// while this one is only reachable by Prometheus on the Docker network.
	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("GET /metrics", metrics.Handler())
		log.Println("metrics on :" + metricsPort)
		log.Fatal(http.ListenAndServe(":"+metricsPort, metricsMux))
	}()

	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, api.Instrument(api.RequirePasswordChange(mux))))
}

// getEnv returns the value of the environment variable key, or fallback if it is unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
