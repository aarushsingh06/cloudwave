
const express = require("express");
const multer = require("multer");
const crypto = require("crypto");
const fs = require("fs/promises");
const path = require("path");

const router = express.Router();

const upload = multer({
  storage: multer.memoryStorage()
});

const CHUNK_SIZE = 1024 * 1024; // 1 MB

const NODES = [
  { id: "node-1", url: "http://localhost:6001" },
  { id: "node-2", url: "http://localhost:6002" },
  { id: "node-3", url: "http://localhost:6003" }
];

const REPLICATION_FACTOR = 2;

const METADATA_PATH = path.join(
  __dirname,
  "../../storage/metadata.json"
);

async function saveMetadata(metadata) {
  await fs.mkdir(path.dirname(METADATA_PATH), {
    recursive: true
  });

  await fs.writeFile(
    METADATA_PATH,
    JSON.stringify(metadata, null, 2)
  );
}

async function distributeChunk(chunk, fileId, index) {
  const primaryIndex = index % NODES.length;

  const selectedNodes = Array.from(
    { length: REPLICATION_FACTOR },
    (_, i) => NODES[(primaryIndex + i) % NODES.length]
  );

  const replicas = [];

  for (const node of selectedNodes) {
    const formData = new FormData();

    formData.append(
      "file",
      new Blob([chunk]),
      `chunk-${fileId}-${index}.bin`
    );

    const response = await fetch(
      `${node.url}/api/files`,
      {
        method: "POST",
        body: formData
      }
    );

    if (!response.ok) {
      throw new Error(`Storage failed on ${node.id}`);
    }

    const result = await response.json();

    replicas.push({
      nodeId: node.id,
      filename: result.filename
    });
  }

  return replicas;
}

router.post("/upload", upload.single("file"), async (req, res) => {
  try {
    if (!req.file) {
      return res.status(400).json({
        error: "Please select a file"
      });
    }

    const fileId = crypto.randomUUID();
    const fileBuffer = req.file.buffer;

    const fileChecksum = crypto
      .createHash("sha256")
      .update(fileBuffer)
      .digest("hex");

    const chunks = [];

    for (
      let offset = 0, index = 0;
      offset < fileBuffer.length;
      offset += CHUNK_SIZE, index++
    ) {
      const chunk = fileBuffer.subarray(
        offset,
        Math.min(offset + CHUNK_SIZE, fileBuffer.length)
      );

      const replicas = await distributeChunk(
        chunk,
        fileId,
        index
      );

      chunks.push({
        index,
        size: chunk.length,
        checksum: crypto
          .createHash("sha256")
          .update(chunk)
          .digest("hex"),
        replicas
      });
    }

    let metadata = [];

    try {
      metadata = JSON.parse(
        await fs.readFile(METADATA_PATH, "utf8")
      );
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }

    const fileMetadata = {
      fileId,
      originalName: req.file.originalname,
      size: req.file.size,
      checksum: fileChecksum,
      chunkSize: CHUNK_SIZE,
      replicationFactor: REPLICATION_FACTOR,
      chunks,
      uploadedAt: new Date().toISOString()
    };

    metadata.push(fileMetadata);
    await saveMetadata(metadata);

    res.status(201).json({
      message: "File distributed successfully",
      file: fileMetadata
    });

  } catch (error) {
    console.error("Distribution error:", error);

    res.status(500).json({
      error: "File distribution failed",
      details: error.message
    });
  }
});

router.get("/download/:fileId", async (req, res) => {
    try {
      const metadata = JSON.parse(
        await fs.readFile(METADATA_PATH, "utf8")
      );

      const file = metadata.find(
        item => item.fileId === req.params.fileId
      );

      if (!file) {
        return res.status(404).json({
          error: "File not found"
        });
      }

      const downloadedChunks = [];

      for (const chunk of file.chunks) {
        let chunkBuffer = null;

        for (const replica of chunk.replicas) {
          const node = NODES.find(
            item => item.id === replica.nodeId
          );

          if (!node) continue;

          try {
            const response = await fetch(
              `${node.url}/api/files/${encodeURIComponent(replica.filename)}`
            );

            if (!response.ok) continue;

            const buffer = Buffer.from(
              await response.arrayBuffer()
            );

            const checksum = crypto
              .createHash("sha256")
              .update(buffer)
              .digest("hex");

            if (checksum === chunk.checksum) {
              chunkBuffer = buffer;
              break;
            }
          } catch (error) {
            console.log(`Replica unavailable: ${replica.nodeId}`);
          }
        }

        if (!chunkBuffer) {
          return res.status(503).json({
            error: `Unable to retrieve chunk ${chunk.index}`
          });
        }

        downloadedChunks.push(chunkBuffer);
      }

      const reconstructedFile = Buffer.concat(downloadedChunks);

      const finalChecksum = crypto
        .createHash("sha256")
        .update(reconstructedFile)
        .digest("hex");

      if (finalChecksum !== file.checksum) {
        return res.status(500).json({
          error: "File integrity verification failed"
        });
      }

      res.setHeader(
        "Content-Type",
        "application/octet-stream"
      );

      res.setHeader(
        "Content-Disposition",
        `attachment; filename="${path.basename(file.originalName)}"`
      );

      res.send(reconstructedFile);

    } catch (error) {
      console.error("Download error:", error);

      res.status(500).json({
        error: "File reconstruction failed"
      });
    }
  });

module.exports = router;
