package main

// installJS runs in OP.GG's main process. It reaches the app's already-loaded
// axios and electron-store singletons through Node's require cache - no webpack
// internals, no module resolution, no breakpoints - and:
//
//   - axios: on the /v2/members/me response, append an ad-free subscription
//     (signed in) or synthesize an ad-free member (signed out).
//   - electron-store, on store-app reads:
//     _ot_v2_member -> always carries the ad-free subscription (in memory).
//     _ot / _ot_v2_refresh -> seeded when empty, so a signed-out client passes
//     hasMemberToken() and runs opggMemberLogin() -> currentMember(), which
//     reads the ad-free member above and writes it to AppStatus. This is what
//     makes the signed-out case work, and it needs no network.
//
// The renderer reads AppStatus.member: member.mid marks it logged in and
// member.features (a string array) containing "ad_free" suppresses the ad view
// and shrinks the window. Real sessions keep their own values (only empties are
// seeded), so signed-in users are unaffected. Nothing is written to disk.
//
// It is idempotent and safe to run repeatedly; returns "ok:<axios>:<store>".
const installJS = `(function(){
  try{
    var req = process.mainModule && process.mainModule.require;
    if(!req) return 'no-main';
    var M = req('module');
    if(!M || !M._cache) return 'no-cache';
    var cache = M._cache;
    var F=['ad_free','premium_badge','lol_mypage','lol_favorites','lol_auto_record','lol_super_renew','tft_mypage','val_mypage','duo_pull_up','duo_highlight'];
    var SUB={plan_id:3,plan_name:'OP.GG Ad-free',state:'active',expiry_at:4102444800,features:F};

    var keys=Object.keys(cache);
    var ax=false, st=false;

    // axios: the cached module whose exports expose an interceptors API.
    for(var i=0;i<keys.length;i++){
      if(keys[i].indexOf('axios')<0) continue;
      var ex=cache[keys[i]].exports; var a=ex&&ex.default||ex;
      if(!(a&&a.interceptors&&a.interceptors.response)) continue;
      if(!a.__opgg){
        a.interceptors.response.use(function(res){
          try{
            var u=res&&res.config&&res.config.url;
            if(typeof u==='string'&&u.split('?')[0].indexOf('/v2/members/me')>=0){
              var b=res.data;
              if(b&&b.code==='SUCCESS'&&b.data&&typeof b.data==='object'){
                var kept=Array.isArray(b.data.subscriptions)?b.data.subscriptions.filter(function(s){return !s||s.plan_id!==3}):[];
                b.data.subscriptions=kept.concat([SUB]);
              }else{
                res.data={code:'SUCCESS',data:{mid:4396,provider:'opgg',nick:'JieJie',email:'Lv-Max',subscriptions:[SUB],scopes:['remember','base']},token:'opgg-ad-patch',refresh_token:'opgg-ad-patch'};
                res.status=200;
              }
            }
          }catch(e){}
          return res;
        });
        a.__opgg=true;
      }
      ax=true; break;
    }

    // electron-store: the cached module whose exports is the Store class.
    for(var j=0;j<keys.length;j++){
      if(keys[j].indexOf('electron-store')<0) continue;
      var se=cache[keys[j]].exports; var S=se&&se.default||se;
      if(!(S&&S.prototype&&S.prototype.get)) continue;
      if(!S.prototype.__opgg){
        var og=S.prototype.get;
        S.prototype.get=function(k,d){
          var v=og.call(this,k,d);
          try{
            if(String(this.path||'').indexOf('store-app')>=0){
              if(k==='_ot_v2_member'){
                var b=(v&&typeof v==='object')?Object.assign({},v):{mid:4396,provider:'opgg',nick:'JieJie',email:'Lv-Max',scopes:['remember','base']};
                var s=Array.isArray(b.subscriptions)?b.subscriptions.filter(function(x){return !x||x.plan_id!==3}):[];
                b.subscriptions=s.concat([SUB]);
                return b;
              }
              // Signed out: seed a session so hasMemberToken() passes and the
              // client runs opggMemberLogin() -> currentMember(), which reads the
              // ad-free member above and writes it to AppStatus (network-free).
              // _ot_v2_refresh is the key hasMemberToken() actually checks; _ot
              // makes the member fetch skip the token-refresh branch. Real
              // sessions keep their own values (only empties are seeded).
              if((k==='_ot'||k==='_ot_v2_refresh') && !v) return 'opgg-ad-patch';
            }
          }catch(e){}
          return v;
        };
        S.prototype.__opgg=true;
      }
      st=true; break;
    }

    if(ax||st) return 'ok:'+ax+':'+st;
    return 'notready';
  }catch(e){return 'ERR:'+(e&&e.message);}
})()`

// closeInspectorJS shuts the debug port once we are done.
const closeInspectorJS = `(function(){ try { process.mainModule.require('inspector').close(); return 'closed'; } catch(e){ return 'noclose:' + (e && e.message); } })()`
