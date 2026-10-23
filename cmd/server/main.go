package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"anypitch/internal/httpapi"
	"anypitch/internal/notification"
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
	go notification.New(conn, os.Getenv("ANYPITCH_WECHAT_NEW_EVENT_TEMPLATE_ID")).Run(context.Background())

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
