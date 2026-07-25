
import os
import sys
import shutil
import subprocess
import tkinter as tk
from tkinter import messagebox
import json
from pathlib import Path

# Native modules (electron-overlay & friends) must stay OUTSIDE the archive.
# Packing them in drops their "unpacked" flag, Electron then dlopens them from a
# lone temp copy, and electron-overlay.node can no longer find its sibling
# n_overlay.x64.dll / injector.exe -> the in-game overlay silently disappears.
UNPACK_GLOB = "*.{node,dll,exe,bat}"

# hook.js lives next to this script; it is the single source of truth for both
# modes, so there is no embedded copy to drift out of sync.
HOOK_JS_PATH = Path(__file__).resolve().parent / "hook.js"


def get_opgg_resources_path():
    """Gets the default resource path for OP.GG."""
    username = os.environ.get("USERNAME")
    return fr"C:\Users\{username}\AppData\Local\Programs\OP.GG\resources"

def check_asar_exists(path):
    return os.path.exists(os.path.join(path, "app.asar"))

def check_backup_exists(path):
    return os.path.exists(os.path.join(path, "app.asar.bak"))

def check_npx_installed():
    """Check if npx is available in the system path."""
    return shutil.which("npx") is not None

def run_asar_extract(src, dest):
    # npx asar extract <archive> <dest>
    # Note: On Windows sometimes 'npx.cmd' is required if not in shell=True, but we use shell=True.
    cmd = f'npx asar extract "{src}" "{dest}"'
    subprocess.run(cmd, shell=True, check=True)

def run_asar_pack(src, dest):
    # npx asar pack <dir> <archive> --unpack <glob>
    cmd = f'npx asar pack "{src}" "{dest}" --unpack "{UNPACK_GLOB}"'
    subprocess.run(cmd, shell=True, check=True)

def restore_backup(resources_path):
    try:
        app_asar = os.path.join(resources_path, "app.asar")
        app_bak = os.path.join(resources_path, "app.asar.bak")
        if os.path.exists(app_asar):
            os.remove(app_asar)
        shutil.copy(app_bak, app_asar)
        return True, "Restored successfully!"
    except Exception as e:
        return False, str(e)

def apply_hook(extract_path, no_login):
    """Adds hook.js as the entry point. The app bundle itself is never touched."""
    if not HOOK_JS_PATH.exists():
        raise Exception(f"hook.js not found next to patcher.py ({HOOK_JS_PATH})")

    main_dir = os.path.join(extract_path, "assets", "main")
    if not os.path.exists(main_dir):
        os.makedirs(main_dir)

    shutil.copy(HOOK_JS_PATH, os.path.join(main_dir, "hook.js"))

    # Marker file that switches hook.js into no-login mode.
    flag_path = os.path.join(main_dir, "nologin.flag")
    if no_login:
        open(flag_path, "w").close()
    elif os.path.exists(flag_path):
        os.remove(flag_path)

    pkg_path = os.path.join(extract_path, "package.json")
    with open(pkg_path, "r", encoding="utf-8") as f:
        data = json.load(f)

    if data.get("main") not in ("assets/main/main.js", "assets/main/hook.js"):
        raise Exception(f"Unexpected entry point: {data.get('main')}")

    data["main"] = "assets/main/hook.js"

    with open(pkg_path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)

def main():
    root = tk.Tk()
    root.title("OP.GG Auto Patcher")
    root.geometry("400x400")
    
    resources_path = get_opgg_resources_path()
    
    lbl_path = tk.Label(root, text=f"Detected Path:\n{resources_path}", wraplength=380, fg="gray")
    lbl_path.pack(pady=10)
    
    status_label = tk.Label(root, text="Checking dependencies...", fg="blue")
    
    def update_status(msg, color="black"):
        status_label.config(text=msg, fg=color)
        root.update()

    def check_env():
        # Check npx first
        if not check_npx_installed():
            messagebox.showerror("Missing Dependency", "Node.js (npx) is not found!\nPlease install Node.js from nodejs.org to use this patcher.")
            return False

        if not os.path.exists(resources_path):
            update_status("OP.GG not found!", "red")
            return False
        if not check_asar_exists(resources_path):
            update_status("app.asar not found!", "red")
            return False
        return True

    def on_restore():
        if not check_env(): return
        if not check_backup_exists(resources_path):
            messagebox.showerror("Error", "No backup found!")
            return
        
        success, msg = restore_backup(resources_path)
        if success:
            messagebox.showinfo("Success", "Restored original app.asar")
            update_status("Restored successfully", "green")
        else:
            messagebox.showerror("Error", f"Restore failed: {msg}")

    def on_patch():
        if not check_env(): return
        
        mode = var_mode.get()
        update_status("Starting patch...", "blue")
        
        app_asar = os.path.join(resources_path, "app.asar")
        app_bak = os.path.join(resources_path, "app.asar.bak")
        temp_dir = os.path.join(resources_path, "temp_patch")
        
        try:
            # Backup
            if not os.path.exists(app_bak):
                update_status("Backing up...", "blue")
                shutil.copy(app_asar, app_bak)
            
            # Extract
            if os.path.exists(temp_dir):
                shutil.rmtree(temp_dir)
            
            # Always extract from the pristine backup, so switching modes or
            # re-patching never stacks a patch on top of a patched archive.
            update_status("Extracting (npx)...", "blue")
            run_asar_extract(app_bak, temp_dir)

            # Patch
            update_status("Patching...", "blue")
            apply_hook(temp_dir, no_login=(mode != "login"))


            # Pack
            update_status("Packing (npx)...", "blue")
            run_asar_pack(temp_dir, app_asar)
            
            # Cleanup
            shutil.rmtree(temp_dir)
            
            update_status("Patch Complete!", "green")
            messagebox.showinfo("Success", f"Patched ({mode} mode) successfully!")
            
        except subprocess.CalledProcessError as e:
            update_status("npx Error!", "red")
            messagebox.showerror("Error", f"ASAR operation failed.\nTry installing the library:\nnpm install -g @electron/asar\n\nDetails: {e}")
            if os.path.exists(temp_dir):
                shutil.rmtree(temp_dir)
        except Exception as e:
            update_status("Error!", "red")
            messagebox.showerror("Error", str(e))
            if os.path.exists(temp_dir):
                shutil.rmtree(temp_dir)

    # UI Controls
    frame_mode = tk.LabelFrame(root, text="Select Mode")
    frame_mode.pack(pady=10, padx=10, fill="x")
    
    var_mode = tk.StringVar(value="login")
    
    tk.Radiobutton(frame_mode, text="Login Version\nRecommended. Uses your own OP.GG account.",
                   variable=var_mode, value="login", justify="left").pack(anchor="w", padx=5, pady=5)

    tk.Radiobutton(frame_mode, text="No-Login Version\nSigns you in as a synthetic account.",
                   variable=var_mode, value="nologin", justify="left").pack(anchor="w", padx=5, pady=5)

    btn_patch = tk.Button(root, text="Patch OP.GG", command=on_patch, bg="#4CAF50", fg="white", font=("Arial", 12, "bold"))
    btn_patch.pack(pady=10, fill="x", padx=20)
    
    btn_restore = tk.Button(root, text="Restore Backup", command=on_restore)
    btn_restore.pack(pady=5)
    
    lbl_note = tk.Label(root, text="Note: Requires Node.js installed.", font=("Arial", 8), fg="gray")
    lbl_note.pack(pady=5)

    status_label.pack(side="bottom", pady=10)

    # Initial check
    if check_npx_installed():
        if check_backup_exists(resources_path):
            update_status("Ready. Backup detected.", "green")
        else:
            update_status("Ready to patch.", "blue")
    else:
        update_status("Node.js (npx) Missing!", "red")

    root.mainloop()

if __name__ == "__main__":
    main()
