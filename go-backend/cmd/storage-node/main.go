
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HealthResponse struct {
	Project   string `json:"project"`
	NodeID    string `json:"nodeId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "node-1"
	}

	port := os.Getenv("NODE_PORT")
	if port == "" {
		port = "6101"
	}

	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = filepath.Join("storage", nodeID)
	}

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(HealthResponse{
			Project:   "CloudWave",
			NodeID:    nodeID,
			Status:    "healthy",
			Timestamp: time.Now().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

			if err := r.ParseMultipartForm(100 << 20); err != nil {
				http.Error(w, "Invalid file upload", http.StatusBadRequest)
				return
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "File is required", http.StatusBadRequest)
				return
			}
			defer file.Close()

			filename := filepath.Base(header.Filename)
			if filename == "." || filename == "" {
				http.Error(w, "Invalid filename", http.StatusBadRequest)
				return
			}

			destination := filepath.Join(storageDir, filename)

			out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				http.Error(w, "Could not save file", http.StatusConflict)
				return
			}

			_, copyErr := io.Copy(out, file)
			closeErr := out.Close()

			if copyErr != nil {
				os.Remove(destination)
				http.Error(w, "Could not store file", http.StatusInternalServerError)
				return
			}

			if closeErr != nil {
				os.Remove(destination)
				http.Error(w, "Could not finish storing file", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(FileInfo{
				Filename: filename,
				Size:     header.Size,
			})

		case http.MethodGet:
			entries, err := os.ReadDir(storageDir)
			if err != nil {
				http.Error(w, "Could not list files", http.StatusInternalServerError)
				return
			}

			files := make([]FileInfo, 0)

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				info, err := entry.Info()
				if err != nil {
					continue
				}

				files = append(files, FileInfo{
					Filename: entry.Name(),
					Size:     info.Size(),
				})
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(files)

		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filename := strings.TrimPrefix(r.URL.Path, "/api/files/")
		if filename == "" || filename != filepath.Base(filename) {
			http.Error(w, "Invalid filename", http.StatusBadRequest)
			return
		}

		http.ServeFile(w, r, filepath.Join(storageDir, filename))
	})

	log.Printf("CloudWave Storage Node %s running on port %s", nodeID, port)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
