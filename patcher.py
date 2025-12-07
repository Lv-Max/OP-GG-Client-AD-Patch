
import os
import sys
import shutil
import subprocess
import tkinter as tk
from tkinter import messagebox
import json
import re
from pathlib import Path

# Embedded hook.js content
HOOK_JS_CONTENT = r"""
const https = require('https');
const originalRequest = https.request;

// Premium Features Configuration
const PREMIUM_DATA = {
    expiry_at: 253402300799, // Far future date
    features: [
        "ad_free", "premium_badge", "lol_mypage", "lol_favorites", 
        "lol_auto_record", "lol_super_renew", "tft_mypage", 
        "val_mypage", "duo_pull_up", "duo_highlight"
    ]
};

console.log('[OPGGHOOK] Hook loaded. Initializing interception...');

https.request = function(...args) {
    let urlOrOptions = args[0];
    let options = args[1] || {};
    let callback = args[2];
    
    // Handle simplified "url, callback" signature or "options, callback"
    if (typeof options === 'function') {
        callback = options;
        options = {};
    }

    let isTarget = false;
    let reqUrl = '';
    let reqHostname = '';

    // Normalize input to find target
    if (typeof urlOrOptions === 'string') {
        if (urlOrOptions.includes('/v2/members/me') || urlOrOptions.includes('member-api.op.gg')) {
            isTarget = true;
            reqUrl = urlOrOptions;
        }
    } else if (typeof urlOrOptions === 'object' && urlOrOptions !== null) {
        reqUrl = urlOrOptions.path || urlOrOptions.pathname || '';
        reqHostname = urlOrOptions.hostname || urlOrOptions.host || '';
        
        if (reqUrl.includes('/v2/members/me') || reqHostname.includes('member-api.op.gg')) {
            isTarget = true;
        }
    }

    if (isTarget) {
        console.log('[OPGGHOOK] Intercepting Member API Request (Object/String matched)');

        // Force identity encoding to avoid gzip/br so we can parse JSON
        if (typeof urlOrOptions === 'object') {
            urlOrOptions.headers = urlOrOptions.headers || {};
            // Remove existing compression headers
            const keys = Object.keys(urlOrOptions.headers);
            for(const k of keys) {
                if(k.toLowerCase() === 'accept-encoding') delete urlOrOptions.headers[k];
            }
            urlOrOptions.headers['Accept-Encoding'] = 'identity';
        } else {
             // If string, we need to merge into options or modify headers if options exists
             if (!options) options = {};
             options.headers = options.headers || {};
             options.headers['Accept-Encoding'] = 'identity';
             args[1] = options; // Update args
        }

        // Intercept response
        const req = originalRequest.apply(this, args);
        
        const originalEmit = req.emit;
        req.emit = function(type, ...emitArgs) {
             if (type === 'response') {
                 const res = emitArgs[0];
                 if (res.statusCode >= 200 && res.statusCode < 300) {
                     console.log('[OPGGHOOK] API Response received (Status:', res.statusCode, '). Buffering data...');
                     
                     const chunks = [];
                     res.on('data', (chunk) => chunks.push(chunk));
                     res.on('end', () => {
                         try {
                             const buffer = Buffer.concat(chunks);
                             let body = buffer.toString('utf8');
                             
                             // Try parsing
                             let json = null;
                             try { json = JSON.parse(body); } catch(e) { console.log('[OPGGHOOK] JSON Parse error (might be raw?):', e.message); }

                             if (json && json.data) {
                                 console.log('[OPGGHOOK] Injecting Premium Data...');
                                 json.data.subscriptions = [PREMIUM_DATA];
                                 
                                 const newBody = JSON.stringify(json);
                                 const modifiedStream = new (require('stream').PassThrough)();
                                 modifiedStream.push(newBody);
                                 modifiedStream.push(null);
                                 
                                 modifiedStream.statusCode = res.statusCode;
                                 modifiedStream.headers = res.headers;
                                 modifiedStream.headers['content-length'] = Buffer.byteLength(newBody);
                                 delete modifiedStream.headers['content-encoding']; // Ensure no encoding remains
                                 
                                 if (callback) callback(modifiedStream);
                                 // Also emit 'response' event on request object for listeners not using callback
                                 originalEmit.call(req, 'response', modifiedStream);
                             } else {
                                 // Not the structure we want, pass through
                                 const pass = new (require('stream').PassThrough)();
                                 pass.push(buffer);
                                 pass.push(null);
                                 Object.assign(pass, res);
                                 if (callback) callback(pass);
                                 originalEmit.call(req, 'response', pass);
                             }
                         } catch (err) {
                             console.error('[OPGGHOOK] Error processing response:', err);
                         }
                     });
                     
                     return true; // Event handled
                 }
             }
             return originalEmit.apply(this, [type, ...emitArgs]);
        };
        
        let userCallback = null;
        if (typeof args[args.length - 1] === 'function') {
            userCallback = args.pop();
        } else if (typeof args[1] === 'function') {
            userCallback = args[1];
            args[1] = undefined; // clear it
        }
        
        const interceptedReq = originalRequest.apply(this, args);
        
        interceptedReq.on('response', (res) => {
             if (res.statusCode >= 200 && res.statusCode < 300) {
                 console.log('[OPGGHOOK] Intercepting Response stream...');
                 const chunks = [];
                 res.on('data', chunk => chunks.push(chunk));
                 res.on('end', () => {
                     const buffer = Buffer.concat(chunks);
                     const str = buffer.toString('utf8');
                     
                     let modified = false;
                     let finalBody = str;

                     try {
                         const json = JSON.parse(str);
                         if (json && json.data) {
                             console.log('[OPGGHOOK] Patching data...');
                             json.data.subscriptions = [PREMIUM_DATA];
                             finalBody = JSON.stringify(json);
                             modified = true;
                         }
                     } catch(e) {
                         console.error('[OPGGHOOK] JSON Parse Failed:', e.message);
                     }

                     // Create a fresh stream for the modified (or original) body
                     const newRes = new (require('stream').PassThrough)();
                     
                     // Copy necessary metadata ONLY. Do NOT copy internal stream state.
                     newRes.statusCode = res.statusCode;
                     newRes.statusMessage = res.statusMessage;
                     newRes.headers = JSON.parse(JSON.stringify(res.headers)); // Deep copy headers
                     newRes.url = res.url;
                     newRes.method = res.method;
                     
                     // Fix headers
                     delete newRes.headers['content-encoding'];
                     if (modified) {
                         newRes.headers['content-length'] = Buffer.byteLength(finalBody);
                     } else {
                         // If we didn't modify, we might still have decompressed it effectively by reading it
                         // so we must ensure content-length matches the buffer we are pushing
                         // But we read it as a buffer, then to string. 
                         // If we just push the buffer back, we are safe.
                         if (!modified) finalBody = buffer; // Use raw buffer if not modified to be safe
                         newRes.headers['content-length'] = Buffer.byteLength(finalBody);
                     }

                     // Push data to the new stream
                     newRes.push(finalBody);
                     newRes.push(null);
                     
                     if (userCallback) userCallback(newRes);
                 });
             } else {
                 if (userCallback) userCallback(res);
             }
        });
        
        return interceptedReq;
    }

    return originalRequest.apply(this, args);
};

// Start the original application
console.log('[OPGGHOOK] Starting original main.js...');
require('./main.js');
"""

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
    # npx asar pack <dir> <archive>
    cmd = f'npx asar pack "{src}" "{dest}"'
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

def patch_login_mode(extract_path):
    """Injects hook.js and updates package.json"""
    main_dir = os.path.join(extract_path, "assets", "main")
    if not os.path.exists(main_dir):
        os.makedirs(main_dir)
    
    # Write hook.js
    with open(os.path.join(main_dir, "hook.js"), "w", encoding="utf-8") as f:
        f.write(HOOK_JS_CONTENT)
        
    # Update package.json
    pkg_path = os.path.join(extract_path, "package.json")
    with open(pkg_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    
    data["main"] = "assets/main/hook.js"
    
    with open(pkg_path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)

def patch_nologin_mode(extract_path):
    """Applies regex replacement used in patch.js"""
    main_js_path = os.path.join(extract_path, "assets", "main", "main.js")
    with open(main_js_path, "r", encoding="utf-8") as f:
        content = f.read()

    patch_count = 0
    
    # 1. Insert Member Data Patch
    user_id_regex = r'(\w+)\.set\("userid",(\w+)\);'
    if re.search(user_id_regex, content):
        def repl(match):
            E_var, X_var = match.groups()
            return f'{E_var}.set("userid",{X_var});{E_var}.set("_ot_v2_member",JSON.parse(\'{{"mid":4396,"provider":"opgg","nick":"JieJie","email":"Lv-Max","subscriptions":[{{"plan_id":3,"plan_name":"OP.GG Ad-free","expiry_at":4092646149,"features":["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"],"state":"expiring"}}],"scopes":["remember","base"],"zendesk_token":"hi"}}\'));{E_var}.set("_ot","TSM");{E_var}.set("_ot_v2_refresh","T1");{E_var}.set("_ot_v2_refresh_at",4092646149000);'
        
        content = re.sub(user_id_regex, repl, content, count=1)
        patch_count += 1
    
    # 2. Features Array Patch
    features_regex = r'features:\((\w+)\.subscriptions\|\|\[\]\)\.reduce\(\(function\(\w+,\w+\)\{.*?\}\),\[\]\)'
    if re.search(features_regex, content):
         content = re.sub(features_regex, 'features:["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"]', content, count=1)
         patch_count += 1
         
    if patch_count > 0:
        with open(main_js_path, "w", encoding="utf-8") as f:
            f.write(content)
        return True
    return False

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
            
            update_status("Extracting (npx)...", "blue")
            run_asar_extract(app_asar, temp_dir)
             
            # Patch
            update_status("Patching...", "blue")
            if mode == "login":
                patch_login_mode(temp_dir)
            else:
                if not patch_nologin_mode(temp_dir):
                    raise Exception("Regex patch failed (patterns not found)")
            
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
    
    tk.Radiobutton(frame_mode, text="Login Version (Hook)\nRecommended. Supports personal accounts.", 
                   variable=var_mode, value="login", justify="left").pack(anchor="w", padx=5, pady=5)
    
    tk.Radiobutton(frame_mode, text="No-Login Version (Regex)\nLegacy. Forces 'JieJie' account.", 
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
