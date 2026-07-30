# OP.GG Client AD Patch

A tiny launcher that removes ads and unlocks the subscriber-only features in the
**OP.GG Desktop Client** — **without modifying any of the app's files**. Because
nothing on disk is touched, it keeps working across the client's auto-updates.

## 📥 Usage

1. Download `OP.GG Launcher.exe` from the [Releases](https://github.com/Lv-Max/OP-GG-Client-AD-Patch/releases) page (or from a workflow run's artifacts).
2. Double-click it.
3. A console window appears for a few seconds. When it prints `patch applied`, the OP.GG client opens with no ads.

Use the launcher (instead of the original OP.GG shortcut) each time you start the app.

- **Signed in or signed out — both work.** You can sign in with your own OP.GG account once (it's remembered) and keep your real profile, or stay signed out; either way the ads are removed.
- If OP.GG is already running, the launcher closes it first (the client's single-instance lock means the patch can only apply to a fresh launch).

## 🛠️ Build

Requires [Go](https://go.dev/) 1.24+.

```bash
cd launcher
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "OP.GG Launcher.exe" .
```

CI builds the same binary on every tag push and manual run (see
[`.github/workflows/build-launcher.yml`](.github/workflows/build-launcher.yml)).

### Source layout

| File | Purpose |
|---|---|
| [`launcher/main.go`](launcher/main.go) | Locate the client, launch it with the inspector, drive the injection, detach. |
| [`launcher/cdp.go`](launcher/cdp.go) | Minimal Chrome DevTools Protocol client over the inspector WebSocket. |
| [`launcher/inject.go`](launcher/inject.go) | The JavaScript hook run inside the client's main process. |
| [`tools/decompress-react-assets.js`](tools/decompress-react-assets.js) | Expand the gzip-compressed renderer bundles for update analysis. |
| [`tools/capture-process-window.ps1`](tools/capture-process-window.ps1) | Capture a client window for layout regression checks. |
| [`tools/capture-electron-page.js`](tools/capture-electron-page.js) | Capture a hidden Electron window through its test-only inspector. |
| [`tools/test-runtime-hook.js`](tools/test-runtime-hook.js) | Exercise signed-out and signed-in hook behavior with Electron API mocks. |

### How the 2.5.2 patch works

OP.GG 2.5.2 no longer publishes a member status during the signed-out startup
path. A store-only hook therefore still works for signed-in accounts but leaves
the signed-out renderer in guest mode.

The launcher now installs two in-memory hooks:

1. The `store-app.json` `electron-store` instance reports the real account
   (when present) plus an active ad-free subscription. It reports a synthetic
   member when signed out; other stores such as `store-web.json` are untouched.
2. Each Electron renderer receives the same member through OP.GG's
   `sync-event / app_status` IPC contract. Later guest or member-delete events
   are rewritten so logout cannot restore the ad column.

The inspector listens only on localhost and is closed after the hooks are
installed. `app.asar`, the installation directory, the user's store files, and
the system proxy configuration are not modified.

### Verification

Run `go test ./...` and `go vet ./...` from `launcher/`, then run
`node tools/test-runtime-hook.js` from the repository root. The runtime hook
test covers signed-out, signed-in, delayed-renderer, delete-event, and
non-`store-app` behavior.

## ⚠️ Disclaimer

This project is not affiliated with or endorsed by OP.GG. Use at your own risk.
The maintainer is not responsible for any issues or damages that may occur.

## License

Licensed under the [GNU License](LICENSE).
