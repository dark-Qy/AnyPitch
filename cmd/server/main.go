package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"anypitch/internal/httpapi"
)

func main() {
	dbPath := envOrDefault("APP_DB_PATH", filepath.Join("data", "anypitch.db"))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer conn.Close()

	handler, err := httpapi.NewHandler(conn)
	if err != nil {
		log.Fatalf("create handler: %v", err)
	}

	addr := envOrDefault("HTTP_ADDR", "127.0.0.1:8080")
	log.Printf("AnyPitch listening on http://%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
