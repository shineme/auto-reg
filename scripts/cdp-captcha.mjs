import fs from "node:fs/promises";
import path from "node:path";

const cdpPort = Number(process.env.CDP_PORT || 9333);
const cdpHost = process.env.CDP_HOST || "127.0.0.1";
const action = process.env.CAPTCHA_ACTION || "send_code";
const targetUrl = process.env.CDP_TARGET_URL || "";
const siteKey = process.env.MINIMAX_RECAPTCHA_SITE_KEY || "";
if (!siteKey || /^<.+>$/.test(siteKey)) throw new Error("MINIMAX_RECAPTCHA_SITE_KEY must be configured locally");
const statePath = process.env.STATE_PATH || path.join(process.cwd(), "artifacts", "minimax-session.json");
const response = await fetch(`http://${cdpHost}:${cdpPort}/json/list`);
const pages = await response.json();
const candidates = pages.filter((entry) => entry.type === "page" && entry.url.includes("account.minimax.io"));
const page = candidates.find((entry) => !targetUrl || entry.url === targetUrl)
  || candidates[0]
  || pages.find((entry) => entry.type === "page");
if (!page) throw new Error("No MiniMax CDP page found");

const socket = new WebSocket(page.webSocketDebuggerUrl);
let nextId = 1;
const pending = new Map();
socket.onmessage = (event) => {
  const message = JSON.parse(event.data);
  if (!message.id || !pending.has(message.id)) return;
  const { resolve, reject } = pending.get(message.id);
  pending.delete(message.id);
  if (message.error) reject(new Error(JSON.stringify(message.error)));
  else resolve(message.result);
};
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = reject;
});

function call(method, params = {}) {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    socket.send(JSON.stringify({ id, method, params }));
  });
}

async function evaluate(expression, awaitPromise = true) {
  const result = await call("Runtime.evaluate", { expression, awaitPromise, returnByValue: true, userGesture: true });
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text || "Runtime.evaluate failed");
  return result.result.value;
}

await call("Page.enable");
await call("Runtime.enable");
await call("Network.enable");
if (targetUrl) {
  await call("Page.navigate", { url: targetUrl });
  await new Promise((resolve) => setTimeout(resolve, 1500));
}
const readyAttempts = Number(process.env.CAPTCHA_READY_ATTEMPTS || 60);
let grecaptchaReady = false;
for (let attempt = 0; attempt < readyAttempts; attempt += 1) {
  grecaptchaReady = await evaluate(`typeof window.grecaptcha !== "undefined" && typeof window.grecaptcha.enterprise !== "undefined"`);
  if (grecaptchaReady) break;
  await new Promise((resolve) => setTimeout(resolve, 500));
}
if (!grecaptchaReady) throw new Error("reCAPTCHA Enterprise did not become ready");
const ticket = await evaluate(`(async () => {
  await new Promise((resolve) => window.grecaptcha.enterprise.ready(resolve));
  return await window.grecaptcha.enterprise.execute("${siteKey}", { action: "${action}" });
})()`);
const cookies = await call("Network.getCookies", { urls: ["https://account.minimax.io/"] });
const localStorage = await evaluate(`JSON.stringify({uuid: localStorage.getItem("mmx_account_UNIQUE_USER_ID"), deviceId: localStorage.getItem("mmx_account_USER_HARD_WARE_INFO")})`);
const state = JSON.parse(await fs.readFile(statePath, "utf8").catch(() => "{}"));
state.cookies ||= {};
state.localStorage ||= {};
for (const cookie of cookies.cookies || []) state.cookies[cookie.name] = cookie.value;
const stored = JSON.parse(localStorage || "{}");
if (stored.uuid) state.localStorage.mmx_account_UNIQUE_USER_ID = stored.uuid;
if (stored.deviceId) state.localStorage.mmx_account_USER_HARD_WARE_INFO = stored.deviceId;
state.captcha = { ticket, randStr: "", captchaType: "recaptcha", action, bridgedAt: new Date().toISOString() };
await fs.writeFile(statePath, JSON.stringify(state, null, 2), "utf8");
console.log(JSON.stringify({ ok: true, statePath, action, ticketLength: ticket.length, cookies: Object.keys(state.cookies), uuid: stored.uuid, deviceId: stored.deviceId }, null, 2));
socket.close();
