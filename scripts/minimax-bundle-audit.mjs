import crypto from "node:crypto";
import fs from "node:fs/promises";
import https from "node:https";
import path from "node:path";

const assetPrefix = process.env.ASSET_PREFIX || "https://cdn.hailuo.ai/mmx-account/prod-web-va-0.1.45";
const outDir = process.env.OUT_DIR || path.join(process.cwd(), "artifacts", "minimax-bundles");

const seedChunks = [
  "static/chunks/webpack-7f03da3d7f5ee22e.js",
  "static/chunks/fd9d1056-6287cd5f33ae32ec.js",
  "static/chunks/117-21409c2be5e25c70.js",
  "static/chunks/main-app-6fca1515cee9a03c.js",
  "static/chunks/50-1f79df9f3d6592a5.js",
  "static/chunks/3-069de7243d23412c.js",
  "static/chunks/app/layout-eff259d96f22be0d.js",
  "static/chunks/app/unified-login/page-4bce7eb65ff4836b.js",
];

const patterns = [
  "device/register",
  "oauth2/login",
  "oauth-authorize",
  "login_redirect",
  "sms/send",
  "email/send",
  "email/verify",
  "loginType",
  "captchaType",
  "randStr",
  "ticket",
  "grecaptcha",
  "enterprise",
  "deviceID",
  "authToken",
  "x-signature",
  "x-timestamp",
  "yy",
  "emailCode",
  "verification",
  "user_code",
  "client_id",
  "mcode-public",
  "sendCode",
  "email",
  "verifyCode",
  "code",
  "confirm",
  "authorize",
];

function get(url) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, { headers: { "User-Agent": "Mozilla/5.0" } }, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(get(new URL(response.headers.location, url).href));
        return;
      }
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => resolve({
        status: response.statusCode,
        body: Buffer.concat(chunks).toString("utf8"),
      }));
    });
    request.setTimeout(20000, () => request.destroy(new Error("request timeout")));
    request.on("error", reject);
  });
}

function chunkNames(source) {
  const names = new Set();
  for (const match of source.matchAll(/static\/chunks\/[A-Za-z0-9_./-]+\.js/g)) {
    names.add(match[0]);
  }
  const webpackMatch = source.match(/h\.u=function\(e\)\{return(.*?)h\.miniCssF=/s);
  if (webpackMatch) {
    const objects = [...webpackMatch[1].matchAll(/\{(?:\d+:"[^"]+",?)+\}/g)].map((match) =>
      Object.fromEntries([...match[0].matchAll(/(\d+):"([^"]+)"/g)].map((entry) => [entry[1], entry[2]])),
    );
    const namesById = objects.find((object) => Object.keys(object).length <= 3) || {};
    const hashesById = objects.find((object) => Object.keys(object).length > 3) || {};
    if (Object.keys(hashesById).length) {
      for (const [id, hash] of Object.entries(hashesById)) {
        const name = namesById[id] || id;
        names.add(`static/chunks/${name}.${hash}.js`);
      }
    }
  }
  return names;
}

function hits(source, needle) {
  const found = [];
  let index = source.indexOf(needle);
  while (index !== -1) {
    found.push(source.slice(Math.max(0, index - 350), Math.min(source.length, index + needle.length + 500)));
    index = source.indexOf(needle, index + needle.length);
    if (found.length >= 3) break;
  }
  return found;
}

await fs.mkdir(outDir, { recursive: true });

const queue = [...seedChunks];
const seen = new Set();
const files = [];

while (queue.length) {
  const chunk = queue.shift();
  if (seen.has(chunk)) continue;
  seen.add(chunk);
  const url = `${assetPrefix}/_next/${chunk}`;
  let response;
  try {
    response = await get(url);
  } catch (error) {
    files.push({ chunk, url, error: error.message });
    continue;
  }
  const hash = crypto.createHash("sha256").update(response.body).digest("hex").slice(0, 12);
  const filename = `${chunk.replace(/[^A-Za-z0-9]+/g, "_")}.${hash}.js`;
  await fs.writeFile(path.join(outDir, filename), response.body, "utf8");
  files.push({ chunk, url, status: response.status, bytes: response.body.length, file: filename });
  if (response.status === 200) {
    for (const name of chunkNames(response.body)) {
      if (!seen.has(name)) queue.push(name);
    }
  }
}

const all = [];
for (const file of files) {
  if (!file.file) continue;
  all.push({ file: file.file, source: await fs.readFile(path.join(outDir, file.file), "utf8") });
}

const report = {
  assetPrefix,
  outDir,
  files,
  matches: Object.fromEntries(patterns.map((pattern) => [
    pattern,
    all.flatMap(({ file, source }) => hits(source, pattern).map((snippet) => ({ file, snippet }))),
  ])),
};

await fs.writeFile(path.join(outDir, "audit.json"), JSON.stringify(report, null, 2), "utf8");
console.log(JSON.stringify({ files: files.length, matches: Object.fromEntries(Object.entries(report.matches).map(([key, value]) => [key, value.length])) }, null, 2));
