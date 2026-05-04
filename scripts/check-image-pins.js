const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const files = [
  "api/Dockerfile",
  "docker-compose.yml",
  "docker-compose.demo.yml",
  "docker-compose.staging.yml",
  "docker-compose.prod.yml",
];

const imageRef = /(?:^|\s)(?:FROM|image:)\s+([^\s]+)/;
const imageArg = /^ARG\s+[A-Z0-9_]*IMAGE=([^\s]+)$/;
const mutableRefs = [];

for (const file of files) {
  const fullPath = path.join(repoRoot, file);
  const lines = fs.readFileSync(fullPath, "utf8").split(/\r?\n/);
  lines.forEach((line, index) => {
    const trimmed = line.trim();
    if (trimmed.startsWith("#") || !trimmed) return;
    const match = trimmed.match(imageRef);
    const argMatch = trimmed.match(imageArg);
    if (!match && !argMatch) return;
    const ref = match ? match[1] : argMatch[1];
    if (ref.startsWith("${")) return;
    if (ref.includes("@sha256:")) return;
    mutableRefs.push(`${file}:${index + 1} uses mutable image reference ${ref}`);
  });
}

if (mutableRefs.length > 0) {
  console.error("Mutable container image references are not allowed:");
  for (const item of mutableRefs) console.error(`- ${item}`);
  process.exit(1);
}
