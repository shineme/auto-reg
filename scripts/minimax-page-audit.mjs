import crypto from "node:crypto";
import fs from "node:fs/promises";
import https from "node:https";
import path from "node:path";

const pageUrl = process.env.PAGE_URL || "https://account.minimax.io/oauth-authorize?client_id=mcode-public&user_code=TEST-CODE";
const outDir = process.env.OUT_DIR || path.join(process.cwd(), "artifacts", "minimax-oauth-page");
const assetPrefix = "https://cdn.hailuo.ai/mmx-account/prod-web-va-0.1.45";

const patterns = [
  "oauth-authorize",
  "user_code",
  "client_id",
  "mcode-public",
  "confirm",
  "approve",
  "authorize",
  "grant",
  "scope",
  "device_code",
  "oauth2",
  "login_redirect",
  "statusInfo",
  "api/user",
  "api/oauth",
  "fetch(",
  "post(",
];

function get(url, headers = {}) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, {
      headers: { "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36", ...headers },
    }, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(get(new URL(response.headers.location, url).href, headers));
        return;
      }
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => resolve({
        status: response.statusCode,
        headers: response.headers,
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
    for (const [id, hash] of Object.entries(hashesById)) {
      names.add(`static/chunks/${namesById[id] || id}.${hash}.js`);
    }
  }
  return names;
}

function endpointCandidates(source) {
  const values = new Set();
  for (const match of source.matchAll(/["'`](\/[^"'`\s]{2,120})["'`]/g)) {
    if (/api|oauth|login|device|verify|grant|authorize|code|confirm|consent|scope|user/.test(match[1])) values.add(match[1]);
  }
  return [...values];
}

function hits(source, needle) {
  const found = [];
  let index = source.indexOf(needle);
  while (index !== -1) {
    found.push(source.slice(Math.max(0, index - 400), Math.min(source.length, index + needle.length + 700)));
    index = source.indexOf(needle, index + needle.length);
    if (found.length >= 4) break;
  }
  return found;
}

await fs.mkdir(outDir, { recursive: true });

const page = await get(pageUrl, { Accept: "text/html,application/xhtml+xml" });
await fs.writeFile(path.join(outDir, "page.html"), page.body, "utf8");

const queue = [...chunkNames(page.body)];
const seen = new Set();
const files = [];

while (queue.length) {
  const chunk = queue.shift();
  if (seen.has(chunk)) continue;
  seen.add(chunk);
  const url = `${assetPrefix}/_next/${chunk}`;
  let response;
  try {
    response = await get(url, { Referer: pageUrl });
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
  pageUrl,
  pageStatus: page.status,
  pageHeaders: page.headers,
  outDir,
  files,
  endpointCandidates: [...new Set(all.flatMap(({ source }) => endpointCandidates(source)))].sort(),
  matches: Object.fromEntries(patterns.map((pattern) => [
    pattern,
    all.flatMap(({ file, source }) => hits(source, pattern).map((snippet) => ({ file, snippet }))),
  ])),
};

await fs.writeFile(path.join(outDir, "audit.json"), JSON.stringify(report, null, 2), "utf8");
console.log(JSON.stringify({
  pageStatus: page.status,
  files: files.length,
  endpointCandidates: report.endpointCandidates,
  matches: Object.fromEntries(Object.entries(report.matches).map(([key, value]) => [key, value.length])),
}, null, 2));
