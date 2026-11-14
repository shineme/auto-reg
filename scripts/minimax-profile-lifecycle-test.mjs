import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const testRoot = path.join(workspace, "artifacts", `profile-unit-${crypto.randomBytes(4).toString("hex")}`);
const batchRoot = path.join(testRoot, "batch");
const batchId = "profile-unit";
const profileRoot = path.join(batchRoot, batchId, "chrome-profiles");
const chromePath = process.env.CHROME_PATH || "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const accountCount = 2;

async function reservePort() {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const port = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return port;
}

async function waitFor(predicate, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("waitFor timed out");
}

async function httpGetJson(url) {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.json();
}

async function processIsAlive(processId) {
  if (!processId) return false;
  if (process.platform === "win32") {
    return new Promise((resolve) => {
      const child = spawn("tasklist.exe", ["/FI", `PID eq ${processId}`, "/NH", "/FO", "CSV"], { stdio: ["ignore", "pipe", "ignore"] });
      let output = "";
      child.stdout.setEncoding("utf8").on("data", (chunk) => { output += chunk; });
      child.once("close", () => resolve(output.includes(`"${processId}"`)));
      child.once("error", () => resolve(false));
    });
  }
  try {
    process.kill(processId, 0);
    return true;
  } catch (error) {
    return error.code === "EPERM";
  }
}

async function main() {
  await fs.rm(testRoot, { recursive: true, force: true });
  await fs.mkdir(testRoot, { recursive: true });
  const firstPort = await reservePort();
  const secondPort = await reservePort();
  assert.notEqual(firstPort, secondPort);
  const launches = [];
  for (const [index, port] of [firstPort, secondPort].entries()) {
    const versionUrl = `http://127.0.0.1:${port}/json/version`;
    const profileDir = path.join(profileRoot, `session-${index + 1}`);
    await fs.mkdir(profileDir, { recursive: true });
    await fs.writeFile(path.join(profileDir, "DevToolsActivePort"), `${port}\n`, "utf8");
    const args = [
      `--remote-debugging-port=${port}`,
      `--user-data-dir=${profileDir}`,
      "--no-first-run",
      "--no-default-browser-check",
      "--disable-background-networking",
      "--window-position=-32000,-32000",
      "about:blank",
    ];
    const chrome = spawn(chromePath, args, { stdio: "ignore", windowsHide: true });
    launches.push({ chrome, profileDir, port });
    try {
      let version;
      await waitFor(async () => {
        try {
          version = await httpGetJson(versionUrl);
          return Boolean(version.webSocketDebuggerUrl);
        } catch {
          return false;
        }
      });
      const socket = new WebSocket(version.webSocketDebuggerUrl);
      await new Promise((resolve, reject) => {
        socket.onopen = resolve;
        socket.onerror = () => reject(new Error("CDP WebSocket failed"));
      });
      let commandId = 1;
      const pending = new Map();
      socket.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (!pending.has(message.id)) return;
        const { resolve, reject } = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) reject(new Error(JSON.stringify(message.error)));
        else resolve(message.result || {});
      };
      const call = (method, params = {}) => new Promise((resolve, reject) => {
        const id = commandId++;
        pending.set(id, { resolve, reject });
        socket.send(JSON.stringify({ id, method, params }));
      });
      const browserContextId = (await call("Target.createBrowserContext", { disposeOnDetach: true })).browserContextId;
      assert.ok(browserContextId);
      await call("Target.disposeBrowserContext", { browserContextId });
      await call("Browser.close");
      socket.close();
    } finally {
      if (chrome.exitCode === null && chrome.signalCode === null) {
        if (process.platform === "win32") {
          await new Promise((resolve) => {
            const killer = spawn("taskkill.exe", ["/PID", String(chrome.pid), "/T", "/F"], { stdio: "ignore", windowsHide: true });
            killer.once("close", resolve);
            killer.once("error", resolve);
          });
        } else {
          chrome.kill("SIGTERM");
        }
      }
      await waitFor(async () => !(await processIsAlive(chrome.pid)), 10000);
      await fs.rm(profileDir, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 });
      assert.equal(await fs.stat(profileDir).then(() => true, () => false), false);
      assert.equal(await processIsAlive(chrome.pid), false);
    }
  }
  const remaining = await fs.readdir(profileRoot, { withFileTypes: true }).catch(() => []);
  assert.equal(remaining.length, 0);
  assert.equal(new Set(launches.map((entry) => entry.port)).size, accountCount);
  assert.equal(new Set(launches.map((entry) => entry.profileDir)).size, accountCount);
  await fs.rm(testRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
  console.log(JSON.stringify({
    ok: true,
    isolatedChromeProfiles: accountCount,
    ephemeralCdpPorts: accountCount,
    browserContextsDisposed: true,
    chromeProcessesStopped: true,
    profilesRemoved: true,
  }));
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message, testRoot }));
  process.exitCode = 1;
});
