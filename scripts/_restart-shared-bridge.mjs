import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const configPath = path.join(process.env.APPDATA, "minimax-registrar", "config.json");
const cfg = JSON.parse(fs.readFileSync(configPath, "utf8"));
const raw = String(cfg.proxyUrl || "").trim().replace(/\\:/g, ":");
const match = /^([a-z][a-z0-9+.-]*):\/\/([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$/i.exec(raw);
const upstream = match
  ? `${match[1]}://${encodeURIComponent(match[2])}:${encodeURIComponent(match[3])}@${match[4]}:${match[5]}${match[6] || ""}`
  : raw;
const urlFile = "C:\\Users\\dhl\\Desktop\\m3\\artifacts\\gui-batch-200\\local-proxy.url";
const exe = "C:\\Users\\dhl\\Desktop\\m3\\app\\cmd\\proxy-bridge-server\\proxy-bridge-server.exe";
fs.rmSync(urlFile, { force: true });
const child = spawn(exe, [upstream, urlFile], { detached: true, stdio: "ignore", windowsHide: true });
child.unref();
const deadline = Date.now() + 20000;
let local = "";
while (Date.now() < deadline) {
  try {
    local = fs.readFileSync(urlFile, "utf8").trim();
    if (local) break;
  } catch {}
  await new Promise((resolve) => setTimeout(resolve, 250));
}
if (!local) throw new Error("bridge did not start");
console.log(JSON.stringify({ pid: child.pid, local }));
