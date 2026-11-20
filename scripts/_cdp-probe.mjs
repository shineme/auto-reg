import { spawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const targetUrl = process.argv[2];
const proxyUrl = process.argv[3] || "http://127.0.0.1:58215";
const waitMs = Number(process.argv[4] || 25000);
const chromePath = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const profileDir = path.join(os.tmpdir(), `mmx-cdp-probe-${Date.now()}`);
const port = 9400 + Math.floor(Math.random() * 200);
fs.mkdirSync(profileDir, { recursive: true });
fs.writeFileSync(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");

const child = spawn(chromePath, [
  `--remote-debugging-port=${port}`,
  `--user-data-dir=${profileDir}`,
  "--headless=new",
  "--disable-gpu",
  "--no-first-run",
  "--no-default-browser-check",
  `--proxy-server=${proxyUrl}`,
  "--proxy-bypass-list=*.google.com;*.gstatic.com;*.googleapis.com;*.recaptcha.net",
  "about:blank",
], { stdio: "ignore", windowsHide: true });

const delay = (ms) => new Promise((r) => setTimeout(r, ms));

async function version() {
  for (let i = 0; i < 100; i += 1) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (res.ok) return await res.json();
    } catch {}
    await delay(200);
  }
  throw new Error("no cdp");
}

const versionInfo = await version();
const ws = new WebSocket(versionInfo.webSocketDebuggerUrl);
await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });

let nextId = 1;
const pending = new Map();
const events = [];
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(msg.error.message));
    else resolve(msg.result);
    return;
  }
  if (msg.method === "Network.loadingFailed") events.push({ type: "failed", ...msg.params });
  if (msg.method === "Network.responseReceived") {
    const { response } = msg.params;
    events.push({ type: "response", url: response.url, status: response.status, mime: response.mimeType });
  }
};

function call(method, params = {}, sessionId) {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
  });
}

const { targetId } = await call("Target.createTarget", { url: "about:blank" });
const { sessionId } = await call("Target.attachToTarget", { targetId, flatten: true });
await call("Page.enable", {}, sessionId);
await call("Runtime.enable", {}, sessionId);
await call("Network.enable", {}, sessionId);
await call("Page.navigate", { url: targetUrl }, sessionId);
await delay(waitMs);

const expression = `(function () {
  try {
    const resources = performance.getEntriesByType("resource").map((r) => ({
      name: r.name, duration: Math.round(r.duration), size: r.transferSize, initiator: r.initiatorType
    }));
    return JSON.stringify({
      ready: document.readyState,
      grecaptcha: typeof window.grecaptcha,
      enterprise: typeof (window.grecaptcha && window.grecaptcha.enterprise),
      href: location.href,
      scripts: Array.from(document.scripts).map((s) => s.src).filter(Boolean),
      slowest: resources.sort((a, b) => b.duration - a.duration).slice(0, 12),
      pending: resources.filter((r) => !r.duration || !r.size).slice(0, 12)
    });
  } catch (error) { return JSON.stringify({ error: String(error) }); }
})()`;

const result = await call("Runtime.evaluate", { expression, returnByValue: true }, sessionId);
console.log(result.result.value);
const failed = events.filter((e) => e.type === "failed");
console.log(JSON.stringify({ failed: failed.slice(0, 15), totalEvents: events.length }, null, 2));
ws.close();
child.kill();
