'use strict';

/**
 * OP.GG Client AD Patch - runtime hook.
 *
 * This file replaces `assets/main/main.js` as the Electron entry point
 * (package.json "main"). It patches `electron-store` and then hands control to
 * the untouched original main.js. Nothing in the app bundle is rewritten.
 *
 * Why the store and not the network or the bundle:
 *
 *   member-api.op.gg /v2/members/me
 *          -> AppStore.set('_ot_v2_member', data)          (store-app.json)
 *          -> features = subscriptions.reduce(not-expired -> spread features)
 *          -> WebStore.set('member', {mid, email, features}) (store-web.json)
 *          -> renderer reads member.features to decide whether to draw ads
 *
 * `_ot_v2_member` is the single point every path converges on, and it is a
 * persisted data shape rather than minified code - so this hook does not care
 * what the webpack bundle looks like this week.
 *
 * Two build flavours share this file. A No-Login build ships an empty marker
 * file `nologin.flag` next to it, which additionally synthesises a member for
 * users who never signed in.
 */

const fs = require('fs');
const path = require('path');

const TAG = '[opgg-patch]';

const FEATURES = [
  'ad_free',
  'premium_badge',
  'lol_mypage',
  'lol_favorites',
  'lol_auto_record',
  'lol_super_renew',
  'tft_mypage',
  'val_mypage',
  'duo_pull_up',
  'duo_highlight',
];

// Seconds, not milliseconds - the app compares `1e3 * expiry_at > Date.now()`.
const FAR_FUTURE = 4102444800; // 2100-01-01T00:00:00Z

const PREMIUM_SUBSCRIPTION = {
  plan_id: 3,
  plan_name: 'OP.GG Ad-free',
  state: 'active',
  expiry_at: FAR_FUTURE,
  features: FEATURES,
};

const NO_LOGIN = fs.existsSync(path.join(__dirname, 'nologin.flag'));

// Only used by No-Login builds, and only when nobody is actually signed in.
const SYNTHETIC_MEMBER = {
  mid: 4396,
  provider: 'opgg',
  nick: 'JieJie',
  email: 'Lv-Max',
  scopes: ['remember', 'base'],
};

const isPlainObject = (value) =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

/** electron-store instances are told apart by their backing file. */
function isStore(store, fileName) {
  try {
    return typeof store.path === 'string' && path.basename(store.path) === fileName;
  } catch (error) {
    return false;
  }
}

/** store-app.json `_ot_v2_member`: append an ad-free subscription. */
function withPremiumSubscription(member) {
  const base = isPlainObject(member) ? member : null;

  // Login builds leave a signed-out client completely alone.
  if (!base && !NO_LOGIN) return member;

  const patched = Object.assign({}, base || SYNTHETIC_MEMBER);
  const subscriptions = Array.isArray(patched.subscriptions)
    ? patched.subscriptions.slice()
    : [];

  if (!subscriptions.some((s) => isPlainObject(s) && s.plan_id === PREMIUM_SUBSCRIPTION.plan_id)) {
    subscriptions.push(PREMIUM_SUBSCRIPTION);
  }

  patched.subscriptions = subscriptions;
  return patched;
}

/** store-web.json `member`: the already-reduced shape the renderer consumes. */
function withPremiumFeatures(member) {
  const base = isPlainObject(member) ? member : null;

  if (!base) {
    if (!NO_LOGIN) return member;
    return {
      mid: SYNTHETIC_MEMBER.mid,
      email: SYNTHETIC_MEMBER.email,
      nickname: SYNTHETIC_MEMBER.nick,
      subscriptions: [PREMIUM_SUBSCRIPTION],
      features: FEATURES.slice(),
    };
  }

  // Same rule as above: on a Login build a signed-out client stays untouched.
  if (!NO_LOGIN && !base.mid) return member;

  const current = Array.isArray(base.features) ? base.features : [];
  const merged = Array.from(new Set(current.concat(FEATURES)));
  if (merged.length === current.length) return member;

  return Object.assign({}, base, { features: merged });
}

try {
  const Store = require('electron-store');
  const originalGet = Store.prototype.get;
  const originalSet = Store.prototype.set;

  Object.defineProperty(Store.prototype, 'get', {
    configurable: true,
    writable: true,
    value: function get(key, defaultValue) {
      const value = originalGet.call(this, key, defaultValue);
      try {
        if (key === '_ot_v2_member' && isStore(this, 'store-app.json')) {
          return withPremiumSubscription(value);
        }
        if (key === 'member' && isStore(this, 'store-web.json')) {
          return withPremiumFeatures(value);
        }
        if (NO_LOGIN && key === '_ot_guest' && isStore(this, 'store-app.json')) {
          return false;
        }
        if (NO_LOGIN && key === 'guest' && isStore(this, 'store-web.json')) {
          return false;
        }
      } catch (error) {
        console.error(TAG, 'get hook failed for', key, error);
      }
      return value;
    },
  });

  Object.defineProperty(Store.prototype, 'set', {
    configurable: true,
    writable: true,
    value: function set(key, value) {
      try {
        if (arguments.length >= 2 && key === 'member' && isStore(this, 'store-web.json')) {
          return originalSet.call(this, key, withPremiumFeatures(value));
        }
      } catch (error) {
        console.error(TAG, 'set hook failed for', key, error);
      }
      return originalSet.apply(this, arguments);
    },
  });

  console.log(TAG, `electron-store patched (mode: ${NO_LOGIN ? 'no-login' : 'login'})`);
} catch (error) {
  // Never brick the client over a failed patch - just run unmodified.
  console.error(TAG, 'failed to patch electron-store:', error);
}

require('./main.js');
