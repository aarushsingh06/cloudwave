
const express = require("express");
const cors = require("cors");
require("dotenv").config();

const filesRouter = require("./routes/files");

const app = express();
const PORT = process.env.PORT || 5001;

app.use(cors());
app.use(express.json());
app.use("/api/files", filesRouter);

app.get("/", (req, res) => {
  res.json({
    project: "CloudWave",
    message: "Distributed File Storage System",
    status: "Running"
  });
});

app.get("/api/health", (req, res) => {
  res.json({
    status: "healthy",
    service: "CloudWave Backend",
    timestamp: new Date().toISOString()
  });
});

app.listen(PORT, () => {
  console.log(`CloudWave backend running on port ${PORT}`);
});
