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
