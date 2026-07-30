#!/usr/bin/env node

const fs = require("node:fs");
const path = require("node:path");
const zlib = require("node:zlib");

function usage() {
  console.error(
    "usage: node tools/decompress-react-assets.js <source-dir> <destination-dir>",
  );
  process.exit(2);
}

const [, , sourceArg, destinationArg] = process.argv;
if (!sourceArg || !destinationArg) {
  usage();
}

const sourceDir = path.resolve(sourceArg);
const destinationDir = path.resolve(destinationArg);

if (!fs.statSync(sourceDir).isDirectory()) {
  throw new Error(`source is not a directory: ${sourceDir}`);
}
if (fs.existsSync(destinationDir)) {
  throw new Error(`destination already exists: ${destinationDir}`);
}

fs.mkdirSync(destinationDir, { recursive: true });

let copied = 0;
let decompressed = 0;

for (const entry of fs.readdirSync(sourceDir, { withFileTypes: true })) {
  if (!entry.isFile()) {
    continue;
  }

  const sourcePath = path.join(sourceDir, entry.name);
  const destinationPath = path.join(destinationDir, entry.name);
  const input = fs.readFileSync(sourcePath);

  if (input.length >= 2 && input[0] === 0x1f && input[1] === 0x8b) {
    fs.writeFileSync(destinationPath, zlib.gunzipSync(input));
    decompressed += 1;
  } else {
    fs.copyFileSync(sourcePath, destinationPath);
    copied += 1;
  }
}

console.log(
  JSON.stringify(
    {
      sourceDir,
      destinationDir,
      decompressed,
      copied,
    },
    null,
    2,
  ),
);
