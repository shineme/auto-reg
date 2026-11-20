import fs from "node:fs";
import path from "node:path";

const file = "C:/Users/dhl/Desktop/m3/scripts/minimax-fast-batch.mjs";
let src = fs.readFileSync(file, "utf8");
const backup = file + ".bak-parallel";
if (!fs.existsSync(backup)) fs.writeFileSync(backup, src);

function replaceOnce(label, from, to) {
  if (!src.includes(from)) throw new Error("missing chunk: " + label);
  src = src.replace(from, to);
  console.log("patched", label);
}

replaceOnce("workers-const",
`const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
`,
`const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
const workers = Math.max(1, Math.min(8, Number(process.env.WORKERS || process.env.MINIMAX_WORKERS || 1) || 1));
const upstreamProxyUrl = process.env.MINIMAX_UPSTREAM_PROXY_URL || "";
const bridgeServerPath = process.env.PROXY_BRIDGE_SERVER || path.join(workspace, "app", "cmd", "proxy-bridge-server", "proxy-bridge-server.exe");
`);

replaceOnce("pending-global",
`const panelCookies = new Map();
const pendingCommands = new Map();
let nextCommandId = 1;
let progressState = {
`,
`const panelCookies = new Map();
const panelQueue = Promise.resolve();
const stateQueue = Promise.resolve();
let progressState = {
`);

replaceOnce("chrome-proxy-arg",
`function chromeProxyArgument() {
  if (!proxyUrl) return "";
  const parsed = new URL(proxyUrl);
`,
`function chromeProxyArgument(localProxy = proxyUrl) {
  if (!localProxy) return "";
  const parsed = new URL(localProxy);
`);

replaceOnce("start-cdp-sig",
`async function startCdpBrowser(profileDir) {
  await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  await fs.mkdir(profileDir, { recursive: true });
  const port = await reserveCdpPort();
  await fs.writeFile(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");
  const args = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profileDir}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-networking",
    "--disable-features=Translate",
    "--disable-component-update",
    "--disable-domain-reliability",
    "--no-pings",
    "--window-position=-32000,-32000",
    `--window-size=${screenWidth},${screenHeight}`,
  ];
  const proxyArgument = chromeProxyArgument();
`,
`async function startCdpBrowser(profileDir, localProxy = proxyUrl) {
  await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  await fs.mkdir(profileDir, { recursive: true });
  const port = await reserveCdpPort();
  await fs.writeFile(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");
  const args = [
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profileDir}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-networking",
    "--disable-features=Translate",
    "--disable-component-update",
    "--disable-domain-reliability",
    "--no-pings",
    "--window-position=-32000,-32000",
    `--window-size=${screenWidth},${screenHeight}`,
  ];
  const proxyArgument = chromeProxyArgument(localProxy);
`);

fs.writeFileSync(file, src);
console.log("phase1 ok", file);
