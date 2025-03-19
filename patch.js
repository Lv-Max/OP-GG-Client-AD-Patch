const fs = require("fs");
const filePath = "extracted/app-64/resources/temp/assets/main/main.js";

let content = fs.readFileSync(filePath, "utf8");

const patches = [
  {
    name: "Member info patch",
    target:
      'var o=function\\(\\){var e;try{var t=m.get\\("_ot_v2_member"\\)\\|\\|{};return{mid:t.mid\\?parseInt\\(t.mid\\):null,email:null!==\\(e=t.email\\)&&void 0!==e\\?e:null,nickname:t.nick,subscriptions:t.subscriptions\\|\\|\\[\\],features:\\(t.subscriptions\\|\\|\\[\\]\\).reduce\\(\\(function\\(e,t\\){return t.expiry_at&&1e3\\*t.expiry_at>Date.now\\(\\)&&\\(e=r\\(r\\(\\[\\],e,!0\\),t.features\\|\\|\\[\\],!0\\)\\),e}\\),\\[\\]\\)}}catch\\(e\\){console.log\\(e\\)}return{mid:null,email:null,nickname:null,subscriptions:\\[\\],features:\\[\\]}\\}\\(\\);',
    replacement:
      'var o={mid:4396,email:"HideOnBush",nickname:"JieJie",subscriptions:["premium"],features:["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"]};',
  },
  {
    name: "Logout function patch",
    target:
      'function w\\(\\){m\\.delete\\("_ot"\\),m\\.delete\\("_ot_member"\\),m\\.delete\\("_ot_v2_refresh"\\),m\\.delete\\("_ot_v2_refresh_at"\\),m\\.delete\\("_ot_v2_member"\\),m\\.delete\\("_ot_v2_member_game_profile"\\),m\\.delete\\("_ot_guest"\\),I\\.delete\\("member"\\),b\\("logout",\\{opgg:!1,opggId:null\\}\\),S\\(\\)}',
    replacement:
      'function w(){/*m.delete("_ot"),m.delete("_ot_member"),m.delete("_ot_v2_refresh"),m.delete("_ot_v2_refresh_at"),m.delete("_ot_v2_member"),m.delete("_ot_v2_member_game_profile"),m.delete("_ot_guest"),I.delete("member"),b("logout",{opgg:!1,opggId:null}),S()*/}',
  },
  {
    name: "Boot sequence patch",
    target:
      "g.whenReady\\(\\).then\\(\\(function\\(\\){\\(f\\|\\|H\\(\\)\\)&&E\\(\\)}\\)\\)",
    replacement:
      'g.whenReady().then((function(){(f||H())&&E();var faker={mid:4396,email:"HideOnBush",nick:"JieJie",subscriptions:["premium"]};m.set("_ot_v2_member",faker);I.set("member",{mid:faker.mid,email:faker.email,nickname:faker.nick,subscriptions:faker.subscriptions,features:["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"]});I.set("guest",!1);S()}))',
  },
];

let patchCount = 0;

patches.forEach((patch) => {
  const regex = new RegExp(patch.target);

  if (content.match(regex)) {
    console.log(`${patch.name}: match found, applying patch...`);
    content = content.replace(regex, patch.replacement);
    patchCount++;
  } else {
    console.error(`${patch.name}: target not found.`);
  }
});

if (patchCount > 0) {
  fs.writeFileSync(filePath, content);
  console.log(`Total patches applied: ${patchCount}`);
} else {
  console.error("No patches applied. Check patterns.");
}
