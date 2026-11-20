import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";

const workspace = "C:/Users/dhl/Desktop/m3";
const configPath = path.join(process.env.APPDATA || "", "minimax-registrar", "config.json");
const config = JSON.parse(await fs.readFile(configPath, "utf8"));

function normalizeProxyUrl(raw) {
  let value = String(raw || "").trim().replace(/\\:/g, ":");
  const compact = /^([a-z][a-z0-9+.-]*):\/\/([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$/i.exec(value);
  if (compact) {
    value = `${compact[1]}://${encodeURIComponent(compact[2])}:${encodeURIComponent(compact[3])}@${compact[4]}:${compact[5]}${compact[6] || ""}`;
  }
  return value;
}

const upstream = normalizeProxyUrl(config.proxyUrl);
if (!upstream) throw new Error("config.proxyUrl is empty");

const exe = path.join(workspace, "app/cmd/proxy-bridge-server/proxy-bridge-server.exe");
const workDir = path.join(workspace, "artifacts/gui-batch-200");
const count = Number(process.argv[2] || 2);

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function runCurl(proxyUrl) {
  return new Promise((resolve) => {
    const child = spawn("curl.exe", ["-s", "--max-time", "30", "-x", proxyUrl, "https://api.ipify.org"], {
      windowsHide: true,
    });
    let out = "";
    let err = "";
    child.stdout.on("data", (chunk) => { out += chunk.toString("utf8"); });
    child.stderr.on("data", (chunk) => { err += chunk.toString("utf8"); });
    child.on("close", (code) => resolve({ body: out.trim(), code, err: err.trim() }));
  });
}

function probeTcp(localUrl) {
  return new Promise((resolve) => {
    const parsed = new URL(localUrl);
    const socket = net.connect({ host: parsed.hostname, port: Number(parsed.port) });
    socket.setTimeout(5000);
    socket.once("connect", () => { socket.destroy(); resolve("open"); });
    socket.once("timeout", () => { socket.destroy(); resolve("timeout"); });
    socket.once("error", (error) => resolve(`error:${error.code || error.message}`));
  });
}

function rawConnect(localUrl, target = "api.ipify.org:443") {
  return new Promise((resolve) => {
    const parsed = new URL(localUrl);
    const socket = net.connect({ host: parsed.hostname, port: Number(parsed.port) });
    let data = "";
    socket.setTimeout(8000);
    socket.once("connect", () => {
      socket.write(`CONNECT ${target} HTTP/1.1\r\nHost: ${target}\r\n\r\n`);
    });
    socket.on("data", (chunk) => {
      data += chunk.toString("utf8");
      if (data.includes("\r\n\r\n")) {
        socket.destroy();
        resolve(data.split("\r\n")[0]);
      }
    });
    socket.once("timeout", () => { socket.destroy(); resolve(`timeout: ${data.slice(0, 120)}`); });
    socket.once("error", (error) => resolve(`error: ${error.code || error.message}`));
  });
}

const bridges = [];
try {
  for (let index = 1; index <= count; index += 1) {
    const portFile = path.join(workDir, `_ipcheck-${index}-${process.pid}.url`);
    await fs.rm(portFile, { force: true });
    const child = spawn(exe, [upstream, portFile], { windowsHide: true, stdio: "ignore" });
    let localUrl = "";
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      localUrl = await fs.readFile(portFile, "utf8").catch(() => "");
      localUrl = String(localUrl || "").trim();
      if (localUrl.startsWith("http://")) break;
      await delay(100);
    }
    if (!localUrl.startsWith("http://")) throw new Error(`bridge ${index} did not start`);
    const tcp = await probeTcp(localUrl);
    const connectLine = await rawConnect(localUrl);
    bridges.push({ child, portFile, localUrl, tcp, connectLine, pid: child.pid, exitCode: child.exitCode });
  }
  const ips = [];
  const sharedUrl = (await fs.readFile(path.join(workDir, "local-proxy.url"), "utf8").catch(() => "")).trim();
  if (sharedUrl) ips.push({ label: "shared", ...(await runCurl(sharedUrl)) });
  for (const bridge of bridges) ips.push(await runCurl(bridge.localUrl));
  const values = ips.map((item) => item.body);
  console.log(JSON.stringify({
    results: ips,
    distinct: new Set(values.filter(Boolean)).size,
    bridges: bridges.map((item) => ({ url: item.localUrl, tcp: item.tcp, connectLine: item.connectLine, exitCode: item.exitCode, alive: item.child.exitCode === null })),
  }, null, 2));
} finally {
  for (const bridge of bridges) {
    try { process.kill(bridge.child.pid, "SIGTERM"); } catch {}
    await fs.rm(bridge.portFile, { force: true }).catch(() => {});
  }
}
