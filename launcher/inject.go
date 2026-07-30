package main

// installJS uses only stable external Electron APIs and OP.GG's long-standing
// renderer sync-event contract. It does not depend on private webpack module
// ids or edit any on-disk resource.
//
// The store hook makes premium-window/currentMember() see ad_free. The
// webContents hook fixes the 2.5.2 signed-out path, which no longer seeds a
// member in AppStatus, by delivering an in-memory member to every renderer and
// rewriting later guest/member-delete events.
const installJS = `(function(){
  try {
    if (globalThis.__opgg_patch_status) return globalThis.__opgg_patch_status;
    if (
      typeof process === 'undefined' ||
      !process.mainModule ||
      typeof process.mainModule.require !== 'function'
    ) {
      return 'waiting:node-context';
    }
    var nativeRequire = process.mainModule.require.bind(process.mainModule);

    var FEATURES = [
      'ad_free',
      'premium_badge',
      'lol_mypage',
      'lol_favorites',
      'lol_auto_record',
      'lol_super_renew',
      'tft_mypage',
      'val_mypage',
      'duo_pull_up',
      'duo_highlight'
    ];
    var SUBSCRIPTION = {
      plan_id: 3,
      plan_name: 'OP.GG Ad-free',
      state: 'active',
      expiry_at: 4102444800,
      features: FEATURES
    };
    var SYNTHETIC_MEMBER = {
      mid: 4396,
      provider: 'opgg',
      nick: 'OP.GG Member',
      nickname: 'OP.GG Member',
      email: null,
      scopes: ['base']
    };
    var isObject = function(value) {
      return value && typeof value === 'object' && !Array.isArray(value);
    };
    var withSubscription = function(member) {
      var patched = Object.assign({}, isObject(member) ? member : SYNTHETIC_MEMBER);
      var subscriptions = Array.isArray(patched.subscriptions)
        ? patched.subscriptions.filter(function(item) {
            return !item || item.plan_id !== SUBSCRIPTION.plan_id;
          })
        : [];
      patched.subscriptions = subscriptions.concat([SUBSCRIPTION]);
      return patched;
    };
    var toStatusMember = function(member) {
      var raw = withSubscription(member);
      var features = raw.subscriptions.reduce(function(all, subscription) {
        if (
          subscription &&
          subscription.expiry_at &&
          1000 * subscription.expiry_at > Date.now()
        ) {
          return all.concat(subscription.features || []);
        }
        return all;
      }, []);
      return {
        mid: raw.mid ? parseInt(raw.mid, 10) : SYNTHETIC_MEMBER.mid,
        email: raw.email == null ? null : raw.email,
        nickname: raw.nick || raw.nickname || SYNTHETIC_MEMBER.nickname,
        subscriptions: raw.subscriptions,
        features: Array.from(new Set(features.concat(FEATURES)))
      };
    };

    var StoreModule = nativeRequire('electron-store');
    var Store = StoreModule && (StoreModule.default || StoreModule);
    if (!(Store && Store.prototype && typeof Store.prototype.get === 'function')) {
      return 'waiting:electron-store';
    }
    if (!Store.prototype.__opggGet) {
      var originalGet = Store.prototype.get;
      var isAppStore = function(store) {
        return store &&
          typeof store.path === 'string' &&
          /(?:^|[\\/])store-app\.json$/i.test(store.path);
      };
      Store.prototype.get = function(key) {
        var value = originalGet.apply(this, arguments);
        if (!isAppStore(this)) return value;
        if (key === '_ot_v2_member') return withSubscription(value);
        if (key === '_ot_guest') return false;
        return value;
      };
      Object.defineProperty(Store.prototype, '__opggGet', { value: true });
    }
    var appStore = new Store({
      name: 'store-app',
      clearInvalidConfig: true,
      accessPropertiesByDotNotation: true
    });

    var electron = nativeRequire('electron');
    var app = electron.app;
    var webContentsModule = electron.webContents;
    if (!(app && typeof app.on === 'function' && webContentsModule)) {
      return 'waiting:electron';
    }

    var currentMember = function() {
      return toStatusMember(appStore.get('_ot_v2_member'));
    };
    var rewriteSyncEvent = function(args) {
      if (args[0] !== 'app_status' || !isObject(args[2])) return args;
      var operation = args[1];
      var change = args[2];
      if (change.key === 'member') {
        if (operation === 'delete') {
          args[1] = 'set';
          args[2] = {
            key: 'member',
            value: currentMember(),
            oldValue: change.oldValue
          };
        } else if (operation === 'set') {
          args[2] = Object.assign({}, change, {
            value: toStatusMember(change.value)
          });
        }
      } else if (change.key === 'guest') {
        if (operation === 'delete') args[1] = 'set';
        args[2] = Object.assign({}, change, { value: false });
      }
      return args;
    };
    var pushMember = function(contents) {
      if (!contents || contents.isDestroyed()) return;
      try {
        var member = currentMember();
        contents.send('sync-event', 'app_status', 'set', {
          key: 'member',
          value: member,
          oldValue: null
        });
        contents.send('sync-event', 'app_status', 'set', {
          key: 'guest',
          value: false,
          oldValue: true
        });
        globalThis.__opgg_patch_last_push =
          member.mid + ':' + member.features.length + ':' + contents.id;
      } catch (error) {}
    };
    var patchContents = function(contents) {
      if (!contents || contents.__opggSend || typeof contents.send !== 'function') return;
      var originalSend = contents.send;
      contents.send = function(channel) {
        var args = Array.prototype.slice.call(arguments, 1);
        if (channel === 'sync-event') args = rewriteSyncEvent(args);
        return originalSend.apply(this, [channel].concat(args));
      };
      Object.defineProperty(contents, '__opggSend', { value: true });
      contents.on('did-finish-load', function() {
        [0, 150, 500, 1500, 4000].forEach(function(delay) {
          setTimeout(function() { pushMember(contents); }, delay);
        });
      });
      if (!contents.isLoading()) {
        [0, 150, 500].forEach(function(delay) {
          setTimeout(function() { pushMember(contents); }, delay);
        });
      }
    };

    app.on('web-contents-created', function(event, contents) {
      patchContents(contents);
    });
    webContentsModule.getAllWebContents().forEach(patchContents);

    var seeded = currentMember();
    globalThis.__opgg_patch_status =
      'applied:store+renderer:' + seeded.mid + ':' + seeded.features.length;
    return globalThis.__opgg_patch_status;
  } catch (error) {
    return 'error:' + (error && error.message);
  }
})()`

const patchStatusJS = `(function(){
  return (globalThis.__opgg_patch_status || 'not-applied') +
    ':push=' + (globalThis.__opgg_patch_last_push || 'pending');
})()`

const closeInspectorJS = `(function(){
  try {
    var inspector = process.mainModule.require('inspector');
    setTimeout(function() { try { inspector.close(); } catch (error) {} }, 250);
    return 'scheduled';
  } catch (error) {
    return 'noclose:' + (error && error.message);
  }
})()`
