# CloudWave

**A Fault-Tolerant Distributed File Storage System Built with Go**

CloudWave is a distributed file storage project designed to store files across multiple storage nodes. Instead of keeping an entire file on one node, the system divides files into smaller chunks, distributes them across storage nodes and maintains replicas to improve availability.

The project explores core distributed systems concepts such as data partitioning, replication, integrity verification and fault tolerance.

## Current Features

- **File Chunking:** Splits uploaded files into 1 MB chunks.
- **Distributed Storage:** Distributes chunks across three storage-node processes.
- **Data Replication:** Maintains two copies of each chunk.
- **Fault Tolerance:** Attempts another replica when a storage node is unavailable.
- **SHA-256 Verification:** Checks chunk integrity and verifies the reconstructed file.
- **File Reconstruction:** Reassembles downloaded chunks in their original order.
- **REST APIs:** Provides HTTP endpoints for coordinator and storage-node communication.

## Architecture

CloudWave currently uses a coordinator-based architecture.

```text
                  Client
                    |
                    v
             Go Coordinator
                Port 7001
                    |
          Chunking and Replication
                    |
          +---------+---------+
          |         |         |
          v         v         v
       Node 1    Node 2    Node 3
       :6101     :6102     :6103
```

Each file is divided into chunks of 1 MB. Chunks are distributed using a round-robin placement strategy, with two replicas stored on different nodes.

For example:

| Chunk | Replica 1 | Replica 2 |
|---|---|---|
| Chunk 1 | Node 1 | Node 2 |
| Chunk 2 | Node 2 | Node 3 |
| Chunk 3 | Node 3 | Node 1 |

If one replica becomes unavailable during download, the coordinator attempts to retrieve the chunk from its alternate replica.

## Technology Stack

| Component | Technology |
|---|---|
| Backend | Go (Golang) |
| Communication | REST APIs / HTTP |
| File Integrity | SHA-256 |
| Frontend | React (planned integration) |
| Version Control | Git and GitHub |
| Containerization | Docker (planned) |

## Project Structure

```text
CloudWave/
├── backend/                 # Original Node.js reference implementation
├── go-backend/
│   ├── cmd/
│   │   ├── coordinator/
│   │   │   ├── main.go
│   │   │   └── files.go
│   │   └── storage-node/
│   │       └── main.go
│   ├── go.mod
│   ├── storage/             # Runtime storage
│   └── data/                # Runtime metadata
├── frontend/                # Planned React frontend
├── infrastructure/          # Planned deployment configuration
├── docs/                    # Project documentation
└── README.md
```

## Running Locally

### Requirements

- Go 1.24.2 or later
- curl
- Git

### 1. Start the Storage Nodes

Open three separate terminal windows from the CloudWave project directory.

**Node 1**

```bash
cd go-backend
NODE_ID=node-1 NODE_PORT=6101 go run ./cmd/storage-node
```

**Node 2**

```bash
cd go-backend
NODE_ID=node-2 NODE_PORT=6102 go run ./cmd/storage-node
```

**Node 3**

```bash
cd go-backend
NODE_ID=node-3 NODE_PORT=6103 go run ./cmd/storage-node
```

### 2. Start the Coordinator

Open another terminal from the CloudWave project directory.

```bash
cd go-backend
go run ./cmd/coordinator
```

The coordinator runs on port `7001`.

### 3. Check Service Health

Check the coordinator:

```bash
curl http://localhost:7001/api/health
```

Check individual storage nodes:

```bash
curl http://localhost:6101/health
curl http://localhost:6102/health
curl http://localhost:6103/health
```

## API Endpoints

### Coordinator

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/api/health` | Check coordinator health |
| POST | `/api/files/upload` | Upload and distribute a file |
| GET | `/api/files/download/{fileId}` | Reconstruct and download a file |

### Storage Node

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/health` | Check node health |
| POST | `/api/files` | Store a chunk |
| GET | `/api/files` | List stored chunks |
| GET | `/api/files/{filename}` | Retrieve a stored chunk |

### Example: Upload a File

Create a sample file:

```bash
printf "Hello CloudWave!" > /tmp/cloudwave-test.txt
```

Upload it:

```bash
curl -X POST \
  -F "file=@/tmp/cloudwave-test.txt" \
  http://localhost:7001/api/files/upload
```

The coordinator returns a response containing the generated file ID.

### Example: Download a File

Replace `YOUR_FILE_ID` with the ID returned by the upload request.

```bash
curl -f -o /tmp/cloudwave-downloaded.txt \
  http://localhost:7001/api/files/download/YOUR_FILE_ID
```

The downloaded file is reconstructed from its stored chunks and verified using SHA-256.

## Testing and Verification

The current implementation has been manually tested for:

- Uploading and downloading files.
- Splitting files larger than 1 MB into multiple chunks.
- Maintaining two replicas per chunk.
- Reconstructing downloaded files.
- Comparing original and downloaded file contents.
- Verifying SHA-256 checksums.
- Downloading files after stopping one storage node.

The system successfully reconstructed a 2.56 MB test file and recovered from an individual storage-node failure using its alternate replica.

## Current Limitations

- Storage nodes currently run as separate processes on one machine.
- Metadata is stored locally in a JSON file.
- Replica repair and automatic recovery are not yet implemented.
- Docker deployment is planned.
- The React dashboard is planned.
- Automated integration tests are not yet implemented.

## Future Improvements

- Automated unit and integration tests.
- Automatic replica repair and node recovery.
- Improved metadata management.
- Concurrent upload and download handling.
- Docker Compose deployment.
- React-based monitoring dashboard.
- Performance benchmarking.

## Project Status

**Active Development**

CloudWave is being developed as a practical exploration of distributed storage systems, replication strategies and fault-tolerant backend architecture.

---

**Author:** Aarush Singh

**Repository:** https://github.com/aarushsingh06/CloudWave