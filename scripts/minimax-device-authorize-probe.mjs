import https from "node:https";

const userCode = process.env.USER_CODE || "";
if (!userCode) throw new Error("USER_CODE is required");
const appId = process.env.APP_ID || "3001";
const url = new URL("https://account.minimax.io/oauth2/device/authorize");
url.searchParams.set("user_code", userCode);
url.searchParams.set("app_id", appId);

const response = await new Promise((resolve, reject) => {
  const request = https.get(url, {
    headers: {
      "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36",
      Accept: "application/json, text/plain, */*",
      Referer: `https://account.minimax.io/oauth-authorize?client_id=mcode-public&user_code=${encodeURIComponent(userCode)}`,
      Origin: "https://account.minimax.io",
      "Cache-Control": "no-store",
    },
  }, resolve);
  request.setTimeout(15000, () => request.destroy(new Error("request timeout")));
  request.on("error", reject);
});

let body = "";
for await (const chunk of response) body += chunk;
console.log(JSON.stringify({ status: response.statusCode, headers: response.headers, body: body.slice(0, 4000) }, null, 2));
