const fs = require("fs");
const filePath = "extracted/app-64/resources/temp/assets/main/main.js";

let content = fs.readFileSync(filePath, "utf8");

let patchCount = 0;

let userIdRegex = /(\w+)\.set\("userid",(\w+)\);/;
if (userIdRegex.test(content)) {
  content = content.replace(userIdRegex, (match, E_var, X_var) => {
    console.log(
      `[+] Insert member data: detected variables E=${E_var}, X=${X_var}`
    );
    return `${E_var}.set("userid",${X_var});${E_var}.set("_ot_v2_member",JSON.parse('{"mid":4396,"provider":"opgg","nick":"JieJie","email":"Lv-Max","subscriptions":[{"plan_id":3,"plan_name":"OP.GG Ad-free","expiry_at":4092646149,"features":["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"],"state":"expiring"}],"scopes":["remember","base"],"zendesk_token":"hi"}'));${E_var}.set("_ot","TSM");${E_var}.set("_ot_v2_refresh","T1");${E_var}.set("_ot_v2_refresh_at",4092646149000);`;
  });
  patchCount++;
} else {
  console.error("Insert member data: target not found.");
}

let featuresRegex =
  /features:\((\w+)\.subscriptions\|\|\[\]\)\.reduce\(\(function\(\w+,\w+\)\{.*?\}\),\[\]\)/;

if (featuresRegex.test(content)) {
  content = content.replace(
    featuresRegex,
    'features:["ad_free","premium_badge","lol_mypage","lol_favorites","lol_auto_record","lol_super_renew","tft_mypage","val_mypage","duo_pull_up","duo_highlight"]'
  );
  patchCount++;
  console.log("[+] Features array replaced successfully.");
} else {
  console.error("Replace features array: target not found.");
}

if (patchCount > 0) {
  fs.writeFileSync(filePath, content);
  console.log(`Total patches applied: ${patchCount}`);
} else {
  console.error("No patches applied. Check patterns carefully.");
}
