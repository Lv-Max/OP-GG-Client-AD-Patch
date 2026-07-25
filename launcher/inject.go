package main

// installJS runs in OP.GG's main process after the launcher has stashed the
// bundle's __webpack_require__ on globalThis. It marks the member as ad-free;
// the renderer then never creates the ad element, so no ad loads. Works signed
// in (keeps the real account) or signed out (seeds a member).
const installJS = `(function(){
  try {
    var wr = globalThis.__opgg_wr;
    if (!wr) return 'no-wr';
    // Find the module exporting AppStore/AppStatus by shape, not a fixed id.
    var mod = null;
    try {
      var cache = wr.c || {};
      for (var k in cache) {
        var ex = cache[k] && cache[k].exports;
        if (ex && ex.AppStatus && ex.AppStore && ex.WebStore) { mod = ex; break; }
      }
    } catch (e) {}
    if (!mod) { try { mod = wr(8699); } catch (e) {} } // fallback to the known id
    if (!mod || !mod.AppStatus) return 'no-appstatus';
    var AS = mod.AppStatus, STORE = mod.AppStore;
    var F = ['ad_free','premium_badge','lol_mypage','lol_favorites','lol_auto_record','lol_super_renew','tft_mypage','val_mypage','duo_pull_up','duo_highlight'];
    var SUB = { plan_id: 3, plan_name: 'OP.GG Ad-free', state: 'active', expiry_at: 4102444800, features: F };
    var withSub = function(m){
      var b = (m && typeof m === 'object') ? Object.assign({}, m) : { mid: 4396, provider: 'opgg', nick: 'JieJie', email: 'Lv-Max', scopes: ['remember','base'] };
      var s = Array.isArray(b.subscriptions) ? b.subscriptions.filter(function(x){ return !x || x.plan_id !== 3; }) : [];
      b.subscriptions = s.concat([SUB]);
      return b;
    };
    // Make the store report the ad-free subscription (in memory only) so the
    // window also shrinks to the ad-free layout.
    if (STORE && STORE.get && !STORE.__opggGet) {
      var og = STORE.get.bind(STORE);
      STORE.get = function(k,d){ var v = og(k,d); return k === '_ot_v2_member' ? withSub(v) : v; };
      STORE.__opggGet = true;
    }
    // Any member the app stores keeps ad_free in its features.
    if (AS.set && !AS.__opggSet) {
      var os = AS.set.bind(AS);
      AS.set = function(k,v){
        if (k === 'member' && v && typeof v === 'object') {
          v = Object.assign({}, v, { features: Array.from(new Set((Array.isArray(v.features) ? v.features : []).concat(F))) });
        }
        return os(k,v);
      };
      AS.__opggSet = true;
    }
    // Signed out: no member exists, so seed one.
    var cur = AS.get('member');
    if (!cur || !cur.mid) { var syn = withSub(null); syn.features = F.slice(); AS.set('member', syn); AS.set('guest', false); }
    var g = AS.get('member');
    return 'ok:' + (g && g.mid) + ':' + ((g && g.features || []).length);
  } catch (e) { return 'ERR:' + (e && e.message); }
})()`

// closeInspectorJS shuts the debug port once we are done.
const closeInspectorJS = `(function(){ try { process.mainModule.require('inspector').close(); return 'closed'; } catch(e){ return 'noclose:' + (e && e.message); } })()`

// Start of the webpack require function; the launcher breaks just inside it.
const webpackRequireAnchor = "function __webpack_require__(e){"
