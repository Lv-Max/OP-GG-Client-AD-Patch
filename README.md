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
