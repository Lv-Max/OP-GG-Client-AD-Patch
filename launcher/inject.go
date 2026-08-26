package main

// installJS is evaluated inside the client's main process. It is idempotent,
// returns "ok:<a>:<b>:<c>", writes nothing to disk, and leaves real sessions as-is.
const installJS = `(function(){
  try{
    var req = process.mainModule && process.mainModule.require;
    if(!req) return 'no-main';
    var M = req('module');
    if(!M || !M._cache) return 'no-cache';
    var cache = M._cache;
    var F=['ad_free','premium_badge','lol_mypage','lol_favorites','lol_auto_record','lol_super_renew','tft_mypage','val_mypage','duo_pull_up','duo_highlight'];
    var SUB={plan_id:3,plan_name:'OP.GG Ad-free',state:'active',expiry_at:4102444800,features:F};

    // The shape AppStatus holds: what the renderer reads for the ad gate.
    function member(m){
      var b=(m&&typeof m==='object')?Object.assign({},m):{};
      if(!b.mid){b.mid=4396;b.email=b.email||'Lv-Max';b.nickname=b.nickname||'JieJie';}
      var s=Array.isArray(b.subscriptions)?b.subscriptions.filter(function(x){return !x||x.plan_id!==3}):[];
      b.subscriptions=s.concat([SUB]);
      var f=Array.isArray(b.features)?b.features.slice():[];
      for(var i=0;i<F.length;i++) if(f.indexOf(F[i])<0) f.push(F[i]);
      b.features=f;
      return b;
    }

    var keys=Object.keys(cache);
    var ax=false, st=false, ipc=false;

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
              if((k==='_ot'||k==='_ot_v2_refresh') && !v) return 'opgg-ad-patch';
            }
          }catch(e){}
          return v;
        };
        S.prototype.__opgg=true;
      }
      st=true; break;
    }

    // The store hooks only feed the app's own reads. The renderer learns the
    // member over IPC, and a logout (or a failed token refresh) pushes a
    // delete that would strip it back to an ad-supported session. Rewrite that
    // one message so the ad-free member survives, and keep the AppStatus
    // snapshot in step so a renderer reload sees the same thing.
    var el = req('electron');

    function status(){
      try{
        var ls = el.ipcMain.listeners('sync');
        for(var i=0;i<ls.length;i++){
          var f={}; ls[i](f,'app_status','init');
          if(f.returnValue && typeof f.returnValue==='object') return f.returnValue;
        }
      }catch(e){}
      return null;
    }

    function hookSend(p){
      if(!(p && typeof p.send==='function') || p.__opgg) return false;
      var send=p.send;
      p.send=function(ch,type,op,d){
        try{
          if(ch==='sync-event' && type==='app_status' && d && d.key==='member' && (op==='delete'||op==='set')){
            var m=member(op==='delete'?null:d.value);
            var L=status(); if(L) L.member=m;
            return send.call(this,ch,type,'set',{key:'member',value:m,oldValue:d.oldValue});
          }
        }catch(e){}
        return send.apply(this,arguments);
      };
      p.__opgg=true;
      return true;
    }

    try{
      var wcs = el.webContents && el.webContents.getAllWebContents ? el.webContents.getAllWebContents() : [];
      if(wcs.length){
        hookSend(Object.getPrototypeOf(wcs[0]));
        ipc=true;
      }else if(el.app && !el.app.__opgg){
        // No window yet: catch the prototype the moment one exists.
        el.app.on('web-contents-created',function(e,wc){ try{ hookSend(Object.getPrototypeOf(wc)); }catch(x){} });
        el.app.__opgg=true;
        ipc=true;
      }else if(el.app && el.app.__opgg){
        ipc=true;
      }
    }catch(e){}

    if(ax||st) return 'ok:'+ax+':'+st+':'+ipc;
    return 'notready';
  }catch(e){return 'ERR:'+(e&&e.message);}
})()`

// closeInspectorJS shuts the debug port once we are done.
const closeInspectorJS = `(function(){ try { process.mainModule.require('inspector').close(); return 'closed'; } catch(e){ return 'noclose:' + (e && e.message); } })()`
