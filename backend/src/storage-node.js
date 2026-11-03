
const express = require("express");
const multer = require("multer");
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");

const app = express();

const PORT = process.env.NODE_PORT || 6000;
const NODE_ID = process.env.NODE_ID || "node-1";
const STORAGE_DIR = process.env.STORAGE_DIR || path.join(__dirname, "../storage");

fs.mkdirSync(STORAGE_DIR, { recursive: true });

const storage = multer.diskStorage({
  destination: (req, file, cb) => cb(null, STORAGE_DIR),
  filename: (req, file, cb) => {
    const filename = `${crypto.randomUUID()}-${path.basename(file.originalname)}`;
    cb(null, filename);
  }
});

const upload = multer({ storage });

app.get("/health", (req, res) => {
  res.json({
    node: NODE_ID,
    status: "healthy"
  });
});

app.post("/api/files", upload.single("file"), (req, res) => {
  if (!req.file) {
    return res.status(400).json({ error: "No file uploaded" });
  }

  res.status(201).json({
    node: NODE_ID,
    message: "File stored successfully",
    filename: req.file.filename,
    size: req.file.size
  });
});

app.get("/api/files", (req, res) => {
  const files = fs.readdirSync(STORAGE_DIR);

  res.json({
    node: NODE_ID,
    count: files.length,
    files
  });
});

app.get("/api/files/:filename", (req, res) => {
  const filename = path.basename(req.params.filename);

  if (filename !== req.params.filename) {
    return res.status(400).json({ error: "Invalid filename" });
  }

  const filePath = path.join(STORAGE_DIR, filename);

  if (!fs.existsSync(filePath)) {
    return res.status(404).json({ error: "File not found" });
  }

  res.download(filePath);
});

app.listen(PORT, () => {
  console.log(`${NODE_ID} storage node running on port ${PORT}`);
});