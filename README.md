# OP.GG Client AD Patch

Automated ad-removal patcher for the OP.GG Desktop Client. This project removes advertisements and unlocks premium features.

## 📥 Downloads (GitHub Releases)

We provide **two** versions in each [Release](https://github.com/Lv-Max/OP-GG-Client-AD-Patch/releases):

### 1. Login Version

- **File**: `OP.GG-Login-Patched.zip`
- **Features**:
  - Works with your **personal OP.GG account**.
  - Unlocks subscribers-only features.
  - **Requires Login.**

### 2. No-Login Version

- **File**: `OP.GG-NoLogin-Patched.zip`
- **Features**:
  - **No Login Required**.
  - Unlocks subscribers-only features.

## 🔧 How It Works

Both builds add a single file — [`hook.js`](hook.js) — to `app.asar` and point
`package.json`'s `main` at it. The original `main.js` is never rewritten.

At startup the hook wraps `electron-store` and appends an ad-free subscription to
`_ot_v2_member`, which is where every code path in the client converges before
`features` reach the renderer. Because it targets a persisted data shape rather
than minified identifiers, a new OP.GG build cannot break the pattern matching.
Nothing is written to disk, so removing the patch restores the original state.

The repack keeps native modules unpacked (`--unpack`). Without that, Electron
loads `electron-overlay.node` from a lone temp copy where it can no longer find
its sibling `n_overlay.x64.dll` / `injector.exe`, and the in-game overlay
silently stops appearing. `tools/verify-asar-unpacked.js` fails the build if that
ever regresses.

## 🛠️ Local Patcher Tool

You can also patch your own existing installation using the included Python tool.

### Prerequisites

- [Python 3.x](https://www.python.org/downloads/)
- [Node.js](https://nodejs.org/)
  - npm install -g @electron/asar

### Usage

1.  Clone or download this repository.
2.  Run the patcher:
    ```bash
    python patcher.py
    ```
3.  The GUI will open:
    - **Auto-detects** your OP.GG installation path.
    - **Select Mode**: Choose "Login Version" or "No-Login Version".
    - **Patch**: Click to apply.
    - **Restore**: Click to revert changes if needed.

## ⚠️ Disclaimer

This project is not affiliated with or endorsed by OP.GG. Use this patch at your own risk. The maintainer is not responsible for any issues or damages that may occur.

## License

This project is licensed under the [GNU License](LICENSE).
