#!/usr/bin/env node

const fs = require("node:fs");
const http = require("node:http");

if (typeof WebSocket !== "function") {
  console.error("capture-electron-page.js requires Node.js 22 or newer");
  process.exit(2);
}

const port = Number(process.argv[2]);
const destination = process.argv[3];
if (!Number.isInteger(port) || !destination) {
  console.error("usage: capture-electron-page.js <inspector-port> <destination>");
  process.exit(2);
}

function getJSON(path) {
  return new Promise((resolve, reject) => {
    const request = http.get(
      { host: "127.0.0.1", port, path, timeout: 3000 },
      (response) => {
        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => {
          try {
            resolve(JSON.parse(Buffer.concat(chunks).toString("utf8")));
          } catch (error) {
            reject(error);
          }
        });
      },
    );
    request.on("timeout", () => request.destroy(new Error("HTTP timeout")));
    request.on("error", reject);
  });
}

async function main() {
  const targets = await getJSON("/json/list");
  if (!targets[0] || !targets[0].webSocketDebuggerUrl) {
    throw new Error("Node inspector target not found");
  }

  const socket = new WebSocket(targets[0].webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    socket.addEventListener("open", resolve, { once: true });
    socket.addEventListener("error", reject, { once: true });
  });

  const expression = `(async function() {
    var nativeRequire = process.mainModule.require.bind(process.mainModule);
    var electron = nativeRequire('electron');
    var fs = nativeRequire('fs');
    var windows = electron.BrowserWindow.getAllWindows().map(function(window) {
      var bounds = window.getBounds();
      return {
        id: window.id,
        title: window.getTitle(),
        visible: window.isVisible(),
        bounds: bounds,
        url: window.webContents.getURL()
      };
    });
    var candidates = electron.BrowserWindow.getAllWindows().filter(function(window) {
      var bounds = window.getBounds();
      var url = window.webContents.getURL();
      return bounds.height >= 800 &&
        bounds.width >= 1000 &&
        bounds.width <= 1700 &&
        !window.webContents.isDestroyed() &&
        (url.indexOf('/desktop/app/') >= 0 || window.getTitle() === 'opgg-electron-app');
    });
    candidates.sort(function(a, b) {
      var aBounds = a.getBounds();
      var bBounds = b.getBounds();
      return (aBounds.width === 1588 ? -1 : 0) -
        (bBounds.width === 1588 ? -1 : 0);
    });
    if (!candidates[0]) {
      throw new Error('OP.GG main BrowserWindow not found: ' + JSON.stringify(windows));
    }
    var target = candidates[0];
    var image = await target.webContents.capturePage();
    fs.writeFileSync(${JSON.stringify(destination)}, image.toPNG());
    return JSON.stringify({
      captured: {
        id: target.id,
        title: target.getTitle(),
        visible: target.isVisible(),
        bounds: target.getBounds(),
        url: target.webContents.getURL()
      },
      windows: windows
    });
  })()`;

  const result = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("Runtime.evaluate timeout")), 30000);
    socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data);
      if (message.id !== 1) return;
      clearTimeout(timer);
      if (message.error) reject(new Error(JSON.stringify(message.error)));
      else if (message.result && message.result.exceptionDetails) {
        reject(new Error(JSON.stringify(message.result.exceptionDetails)));
      } else resolve(message.result.result.value);
    });
    socket.send(
      JSON.stringify({
        id: 1,
        method: "Runtime.evaluate",
        params: {
          expression,
          awaitPromise: true,
          returnByValue: true,
        },
      }),
    );
  });
  socket.close();

  if (!fs.existsSync(destination)) {
    throw new Error("capture did not create " + destination);
  }
  console.log(result);
}

main().catch((error) => {
  console.error(error.stack || error.message);
  process.exit(1);
});
