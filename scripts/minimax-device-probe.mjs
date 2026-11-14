import crypto from "node:crypto";
import https from "node:https";

const host = "account.minimax.io";
const path = "/v1/api/user/device/register";
const uuid = crypto.randomUUID();
const timestamp = Math.floor(Date.now() / 1000);
const body = JSON.stringify({ uuid });
const requiredEnvironment = (name) => {
  const value = process.env[name] || "";
  if (!value || /^<.+>$/.test(value)) throw new Error(`${name} must be configured locally`);
  return value;
};
const params = {
  device_platform: "web",
  biz_id: "3",
  app_id: "01",
  version_code: "0.1.45",
  unix: Date.now(),
  timezone_offset: -new Date().getTimezoneOffset(),
  sys_language: "en",
  lang: "en",
  uuid,
  device_id: "0",
  os_name: "Windows",
  browser_name: "Chrome",
  device_memory: 8,
  cpu_core_num: 8,
  browser_language: "en-US",
  browser_platform: "Win32",
  user_id: 0,
  screen_width: 1280,
  screen_height: 720,
  client: "web",
};

function md5(value) {
  return crypto.createHash("md5").update(value).digest("hex");
}

const search = new URLSearchParams(params).toString();
const requestPath = `${path}?${search}`;
const signature = md5(`${timestamp}${requiredEnvironment("MINIMAX_SIGNATURE_SALT")}${body}`);
const yy = md5(`${encodeURIComponent(requestPath)}_${body}${md5(String(Date.now()))}${requiredEnvironment("MINIMAX_YY_SUFFIX")}`);

function request() {
  return new Promise((resolve, reject) => {
    const req = https.request(
      {
        host,
        path: requestPath,
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(body),
          "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36",
          Referer: "https://account.minimax.io/unified-login",
          Origin: "https://account.minimax.io",
          Accept: "application/json, text/plain, */*",
          "x-timestamp": String(timestamp),
          "x-signature": signature,
          yy,
        },
        timeout: 15000,
      },
      (response) => {
        let text = "";
        response.on("data", (chunk) => (text += chunk));
        response.on("end", () =>
          resolve({
            status: response.statusCode,
            headers: response.headers,
            body: text.slice(0, 2000),
          }),
        );
      },
    );
    req.on("timeout", () => req.destroy(new Error("request timeout")));
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

console.log(JSON.stringify(await request(), null, 2));
