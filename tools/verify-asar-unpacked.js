/**
 * Guard against the "silently broken overlay" class of bug.
 *
 * `asar pack` without --unpack embeds native modules INSIDE the archive and drops
 * the `unpacked: true` flags from the header. Electron then dlopens *.node from
 * a lone temp copy, so electron-overlay.node can no longer find its sibling
 * n_overlay.x64.dll / injector.exe (it locates them via GetModuleFileNameW).
 * The require() is wrapped in an empty catch upstream, so the in-game overlay
 * just silently never appears.
 *
 * Usage: node tools/verify-asar-unpacked.js <original.asar> <patched.asar>
 * Exits non-zero if any binary that was unpacked in the original is no longer
 * unpacked in the patched archive.
 */
const fs = require("fs");

const BINARY = /\.(node|dll|exe|bat)$/i;

function readHeader(archivePath) {
  const fd = fs.openSync(archivePath, "r");
  try {
    const prefix = Buffer.alloc(16);
    fs.readSync(fd, prefix, 0, 16, 0);
    // [pickle size][uint32 header-pickle size][pickle size][uint32 json length]
    const headerPickleSize = prefix.readUInt32LE(4);
    const jsonLength = prefix.readUInt32LE(12);
    const buf = Buffer.alloc(headerPickleSize + 8);
    fs.readSync(fd, buf, 0, buf.length, 0);
    return JSON.parse(buf.toString("utf8", 16, 16 + jsonLength));
  } finally {
    fs.closeSync(fd);
  }
}

function unpackedFiles(header) {
  const out = new Set();
  const walk = (node, prefix) => {
    for (const [name, entry] of Object.entries(node.files || {})) {
      const full = `${prefix}/${name}`;
      if (entry.files) walk(entry, full);
      else if (entry.unpacked) out.add(full);
    }
  };
  walk(header, "");
  return out;
}

const [, , originalPath, patchedPath] = process.argv;
if (!originalPath || !patchedPath) {
  console.error("usage: verify-asar-unpacked.js <original.asar> <patched.asar>");
  process.exit(2);
}

const original = unpackedFiles(readHeader(originalPath));
const patched = unpackedFiles(readHeader(patchedPath));

const expected = [...original].filter((f) => BINARY.test(f)).sort();
const missing = expected.filter((f) => !patched.has(f));

console.log(`original: ${original.size} unpacked entries (${expected.length} binaries)`);
console.log(`patched : ${patched.size} unpacked entries`);

if (missing.length > 0) {
  console.error(
    `\n[FAIL] ${missing.length} binaries lost their "unpacked" flag in ${patchedPath}:`
  );
  for (const f of missing) console.error(`  - ${f}`);
  console.error(
    "\nRepack with --unpack so Electron keeps loading them from app.asar.unpacked."
  );
  process.exit(1);
}

if (expected.length === 0) {
  console.error(`\n[FAIL] ${originalPath} has no unpacked binaries - unexpected layout.`);
  process.exit(1);
}

console.log(`\n[OK] all ${expected.length} native binaries are still unpacked.`);
