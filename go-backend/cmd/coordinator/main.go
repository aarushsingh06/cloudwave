package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type HealthResponse struct {
	Project   string `json:"project"`
	Service   string `json:"service"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Project:   "CloudWave",
		Service:   "Go Coordinator",
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	if err := loadMetadata(); err != nil {
		log.Fatal("Could not load metadata: ", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/files/upload", uploadHandler)
	mux.HandleFunc("/api/files/download/", downloadHandler)

	log.Println("CloudWave Go Coordinator running on port 7001")

	server := &http.Server{
		Addr:              ":7001",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
