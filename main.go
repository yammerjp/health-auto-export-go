package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			if err := runImport(os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		default:
			log.Fatalf("Unknown command: %s", os.Args[1])
		}
	}

	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "health.db")
	readToken := os.Getenv("READ_TOKEN")
	writeToken := os.Getenv("WRITE_TOKEN")

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv := NewServer(db, readToken, writeToken)
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
