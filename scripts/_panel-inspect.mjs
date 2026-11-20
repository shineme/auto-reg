import fs from "node:fs";
import path from "node:path";

const cfg = JSON.parse(fs.readFileSync(path.join(process.env.APPDATA, "minimax-registrar", "config.json"), "utf8"));
const base = cfg.panelBase.replace(/\/$/, "");
const key = cfg.panelKey;
const cookies = new Map();

function storeCookies(response) {
  for (const raw of response.headers.getSetCookie?.() || []) {
    const [pair] = raw.split(";");
    const index = pair.indexOf("=");
    if (index > 0) cookies.set(pair.slice(0, index), pair.slice(index + 1));
  }
}

async function get(pathname) {
  const headers = new Headers();
  if (cookies.size) headers.set("Cookie", [...cookies].map(([k, v]) => `${k}=${v}`).join("; "));
  const response = await fetch(new URL(pathname, base), { headers, redirect: "manual" });
  storeCookies(response);
  const text = await response.text();
  return { status: response.status, text };
}

const login = await get(`/?k=${encodeURIComponent(key)}&view=providers&edit=minimax-code-global`);
console.log("login", login.status, "cookies", [...cookies.keys()]);

for (const id of ["nWoWviOLp52-", "QfIyDvR8or8W"]) {
  const result = await get(`/api/signin/${id}`);
  console.log(id, result.status, result.text.slice(0, 300));
}

const providers = await get("/api/providers");
const data = JSON.parse(providers.text);
const recent = [];
function walk(value) {
  if (!value || typeof value !== "object") return;
  if (value.agent === "plugin:minimax-code-global" && value.user && value.seen >= "2026-11-17T06:50") {
    recent.push(value);
  }
  for (const child of Object.values(value)) walk(child);
}
walk(data);
console.log("recent", JSON.stringify(recent, null, 2));
