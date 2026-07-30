#!/usr/bin/env node

const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const repositoryRoot = path.resolve(__dirname, "..");
const injectSource = fs.readFileSync(
  path.join(repositoryRoot, "launcher", "inject.go"),
  "utf8",
);
const match = injectSource.match(
  /const installJS = \x60([\s\S]*?)\x60/,
);
if (!match) {
  throw new Error("could not extract installJS from launcher/inject.go");
}
const installJS = match[1];

async function runScenario(persistedMember, { deferredRenderer = false } = {}) {
  class Store {
    constructor(options = {}) {
      this.path = `C:\\test\\${options.name || "config"}.json`;
    }

    get(key) {
      if (key === "_ot_v2_member") {
        return persistedMember;
      }
      if (key === "_ot_guest") {
        return true;
      }
      return undefined;
    }
  }

  const sent = [];
  const contents = new EventEmitter();
  contents.id = 7;
  contents.isDestroyed = () => false;
  contents.isLoading = () => deferredRenderer;
  contents.send = (...args) => sent.push(args);

  const app = new EventEmitter();
  const electron = {
    app,
    webContents: {
      getAllWebContents: () => (deferredRenderer ? [] : [contents]),
    },
  };
  const nativeRequire = (name) => {
    if (name === "electron-store") {
      return Store;
    }
    if (name === "electron") {
      return electron;
    }
    throw new Error(`unexpected require: ${name}`);
  };
  const context = vm.createContext({
    process: {
      mainModule: {
        require: nativeRequire,
      },
    },
    setTimeout,
  });

  const result = vm.runInContext(installJS, context);
  assert.match(result, /^applied:store\+renderer:/);

  if (deferredRenderer) {
    assert.equal(sent.length, 0, "no renderer existed during installation");
    app.emit("web-contents-created", {}, contents);
    contents.emit("did-finish-load");
  }
  await new Promise((resolve) => setTimeout(resolve, 650));

  const memberEvent = sent.find(
    ([channel, target, operation, change]) =>
      channel === "sync-event" &&
      target === "app_status" &&
      operation === "set" &&
      change.key === "member",
  );
  assert.ok(memberEvent, "expected a renderer member event");
  const member = memberEvent[3].value;
  assert.ok(member.features.includes("ad_free"));
  assert.ok(member.features.includes("premium_badge"));

  contents.send("sync-event", "app_status", "delete", {
    key: "member",
    oldValue: member,
  });
  const rewritten = sent.at(-1);
  assert.equal(rewritten[2], "set");
  assert.equal(rewritten[3].key, "member");
  assert.ok(rewritten[3].value.features.includes("ad_free"));

  contents.send("sync-event", "app_status", "delete", {
    key: "guest",
    oldValue: true,
  });
  const rewrittenGuest = sent.at(-1);
  assert.equal(rewrittenGuest[2], "set");
  assert.equal(rewrittenGuest[3].key, "guest");
  assert.equal(rewrittenGuest[3].value, false);

  const store = new Store({ name: "store-app" });
  assert.equal(store.get("_ot_guest"), false);
  assert.ok(
    store
      .get("_ot_v2_member")
      .subscriptions.some((subscription) =>
        subscription.features.includes("ad_free"),
      ),
  );

  const unrelatedStore = new Store({ name: "store-web" });
  assert.equal(unrelatedStore.get("_ot_guest"), true);
  assert.equal(unrelatedStore.get("_ot_v2_member"), persistedMember);

  return member;
}

(async () => {
  const guest = await runScenario(undefined);
  assert.equal(guest.mid, 4396);

  const signedIn = await runScenario({
    mid: 560587,
    email: "member@example.test",
    nick: "Existing user",
    subscriptions: [],
  });
  assert.equal(signedIn.mid, 560587);
  assert.equal(signedIn.nickname, "Existing user");

  const deferredGuest = await runScenario(undefined, {
    deferredRenderer: true,
  });
  assert.equal(deferredGuest.mid, 4396);

  console.log("runtime hook mock tests passed");
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
