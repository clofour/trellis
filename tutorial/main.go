package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

var version = "dev"

func message() string {
	switch version {
	case "v1":
		return "Your first Trellis workload is running."
	case "v2":
		return "Nice — your new application version is running."
	default:
		return "Trellis tutorial workload is running."
	}
}

func main() {
	host, _ := os.Hostname()
	log.Printf("Trellis tutorial · %s", version)
	log.Printf("%s", message())
	log.Printf("allocation host: %s", host)
	log.Printf("started at: %s", time.Now().UTC().Format(time.RFC3339))

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "Trellis tutorial · %s\n\n%s\n", version, message())
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			log.Printf("%s is alive", version)
		}
	}()

	log.Printf("HTTP endpoint ready on :8080 (used in the next tutorial stage)")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
