package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	chunkSize         = 1 * 1024 * 1024
	replicationFactor = 2
	maxFileSize       = 100 * 1024 * 1024
	metadataPath      = "data/metadata.json"
)

type StorageNode struct {
	ID  string
	URL string
}

type ChunkReplica struct {
	NodeID   string `json:"nodeId"`
	Filename string `json:"filename"`
}

type ChunkMetadata struct {
	Index    int            `json:"index"`
	Size     int            `json:"size"`
	Checksum string         `json:"checksum"`
	Replicas []ChunkReplica `json:"replicas"`
}

type FileMetadata struct {
	FileID            string          `json:"fileId"`
	OriginalName      string          `json:"originalName"`
	Size              int64           `json:"size"`
	Checksum          string          `json:"checksum"`
	ChunkSize         int             `json:"chunkSize"`
	ReplicationFactor int             `json:"replicationFactor"`
	Chunks            []ChunkMetadata `json:"chunks"`
	UploadedAt        time.Time       `json:"uploadedAt"`
}

var storageNodes = []StorageNode{
	{ID: "node-1", URL: "http://localhost:6101"},
	{ID: "node-2", URL: "http://localhost:6102"},
	{ID: "node-3", URL: "http://localhost:6103"},
}

var metadata = make(map[string]FileMetadata)
var metadataMutex sync.RWMutex

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func generateID() (string, error) {
	data := make([]byte, 16)

	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	return hex.EncodeToString(data), nil
}

func checksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func saveMetadata() error {
	metadataMutex.RLock()
	data, err := json.MarshalIndent(metadata, "", "  ")
	metadataMutex.RUnlock()

	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return err
	}

	tempPath := metadataPath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, metadataPath)
}

func loadMetadata() error {
	data, err := os.ReadFile(metadataPath)

	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	var loaded map[string]FileMetadata

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	metadataMutex.Lock()
	metadata = loaded
	metadataMutex.Unlock()

	return nil
}

func uploadToNode(node StorageNode, filename string, data []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}

	if _, err := part.Write(data); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodPost,
		node.URL+"/api/files",
		&body,
	)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("node %s returned status %d", node.ID, response.StatusCode)
	}

	return nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize+(1<<20))

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid or oversized upload", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > maxFileSize {
		http.Error(w, "File exceeds 100 MB limit", http.StatusRequestEntityTooLarge)
		return
	}

	fileData, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		http.Error(w, "Could not read file", http.StatusInternalServerError)
		return
	}

	if len(fileData) > maxFileSize {
		http.Error(w, "File exceeds 100 MB limit", http.StatusRequestEntityTooLarge)
		return
	}

	fileID, err := generateID()
	if err != nil {
		http.Error(w, "Could not generate file ID", http.StatusInternalServerError)
		return
	}

	fileMetadata := FileMetadata{
		FileID:            fileID,
		OriginalName:      filepath.Base(header.Filename),
		Size:              int64(len(fileData)),
		Checksum:          checksum(fileData),
		ChunkSize:         chunkSize,
		ReplicationFactor: replicationFactor,
		UploadedAt:        time.Now(),
		Chunks:            []ChunkMetadata{},
	}

	for offset, index := 0, 0; offset < len(fileData); index++ {
		end := offset + chunkSize

		if end > len(fileData) {
			end = len(fileData)
		}

		chunk := fileData[offset:end]
		chunkChecksum := checksum(chunk)

		chunkMetadata := ChunkMetadata{
			Index:    index,
			Size:     len(chunk),
			Checksum: chunkChecksum,
			Replicas: []ChunkReplica{},
		}

		startNode := index % len(storageNodes)

		for replica := 0; replica < replicationFactor; replica++ {
			node := storageNodes[(startNode+replica)%len(storageNodes)]

			replicaID, err := generateID()
			if err != nil {
				http.Error(w, "Could not generate replica ID", http.StatusInternalServerError)
				return
			}

			filename := fmt.Sprintf("%s-chunk-%d-%s.bin", fileID, index, replicaID)

			if err := uploadToNode(node, filename, chunk); err != nil {
				http.Error(w, fmt.Sprintf("Storage failed on %s: %v", node.ID, err), http.StatusBadGateway)
				return
			}

			chunkMetadata.Replicas = append(chunkMetadata.Replicas, ChunkReplica{
				NodeID:   node.ID,
				Filename: filename,
			})
		}

		fileMetadata.Chunks = append(fileMetadata.Chunks, chunkMetadata)
		offset = end
	}

	metadataMutex.Lock()
	metadata[fileID] = fileMetadata
	metadataMutex.Unlock()

	if err := saveMetadata(); err != nil {
		metadataMutex.Lock()
		delete(metadata, fileID)
		metadataMutex.Unlock()

		http.Error(w, "Could not save metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fileMetadata)
}

func getStorageNode(nodeID string) (StorageNode, bool) {
	for _, node := range storageNodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return StorageNode{}, false
}

func downloadChunk(replica ChunkReplica, expectedChecksum string) ([]byte, error) {
	node, exists := getStorageNode(replica.NodeID)
	if !exists {
		return nil, fmt.Errorf("unknown storage node: %s", replica.NodeID)
	}

	request, err := http.NewRequest(
		http.MethodGet,
		node.URL+"/api/files/"+replica.Filename,
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node %s returned status %d", node.ID, response.StatusCode)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if checksum(data) != expectedChecksum {
		return nil, fmt.Errorf("checksum mismatch on %s", node.ID)
	}

	return data, nil
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID := filepath.Base(r.URL.Path)

	metadataMutex.RLock()
	fileMetadata, exists := metadata[fileID]
	metadataMutex.RUnlock()

	if !exists || fileID == "." || fileID == "/" {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var reconstructed bytes.Buffer

	for _, chunk := range fileMetadata.Chunks {
		var chunkData []byte
		var chunkRecovered bool

		for _, replica := range chunk.Replicas {
			data, err := downloadChunk(replica, chunk.Checksum)
			if err == nil {
				chunkData = data
				chunkRecovered = true
				break
			}

			log.Printf("Replica unavailable: %s: %v", replica.NodeID, err)
		}

		if !chunkRecovered {
			http.Error(w, "Could not retrieve a valid chunk replica", http.StatusBadGateway)
			return
		}

		reconstructed.Write(chunkData)
	}

	fileData := reconstructed.Bytes()

	if int64(len(fileData)) != fileMetadata.Size ||
		checksum(fileData) != fileMetadata.Checksum {
		http.Error(w, "Reconstructed file integrity check failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(
		"attachment",
		map[string]string{"filename": fileMetadata.OriginalName},
	))
	w.Header().Set("Content-Length", strconv.Itoa(len(fileData)))
	w.WriteHeader(http.StatusOK)
	w.Write(fileData)
}
