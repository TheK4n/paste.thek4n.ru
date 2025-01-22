package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const DEFAULT_PORT = 80

func main() {
	log.Println("Connecting to database...")

	db, err := storage.InitStorageDB()
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	handlers := handlers.Handlers{Db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping/", handlers.Pingpong)
	mux.HandleFunc("GET /{key}/", handlers.Get)
	mux.HandleFunc("POST /", handlers.Cache)

	port, err := portFromEnvOrDefault()
	if err != nil {
		log.Fatalf("Invalid port: %s", err.Error())
	}

	hostport := fmt.Sprintf("0.0.0.0:%d", port)

	log.Printf("Server started on %s ...", hostport)

	log.Fatal(http.ListenAndServe(hostport, mux))
}

func portFromEnvOrDefault() (int, error) {
	portEnv := os.Getenv("PASTE_PORT")

	if portEnv == "" {
		return DEFAULT_PORT, nil
	}

	return strconv.Atoi(portEnv)
}
