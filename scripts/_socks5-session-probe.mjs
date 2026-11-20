import fs from "node:fs/promises";
import net from "node:net";
import path from "node:path";

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

const upstream = new URL(normalizeProxyUrl(config.proxyUrl));
const baseUser = decodeURIComponent(upstream.username);
const password = decodeURIComponent(upstream.password);
const host = upstream.hostname;
const port = Number(upstream.port || 1080);

function socksConnect(user, targetHost, targetPort) {
  return new Promise((resolve) => {
    const socket = net.connect({ host, port });
    let stage = 0;
    let buffer = Buffer.alloc(0);
    const fail = (message) => { socket.destroy(); resolve({ ok: false, error: message }); };
    socket.setTimeout(20000, () => fail("timeout"));
    socket.on("error", (error) => resolve({ ok: false, error: error.code || error.message }));
    socket.on("connect", () => {
      socket.write(Buffer.from([0x05, 0x02, 0x00, 0x02]));
    });
    socket.on("data", (chunk) => {
      buffer = Buffer.concat([buffer, chunk]);
      if (stage === 0) {
        if (buffer.length < 2) return;
        const method = buffer[1];
        buffer = buffer.subarray(2);
        if (method !== 0x02) return fail(`auth method rejected: ${method}`);
        const userBuf = Buffer.from(user, "utf8");
        const passBuf = Buffer.from(password, "utf8");
        socket.write(Buffer.concat([Buffer.from([0x01, userBuf.length]), userBuf, Buffer.from([passBuf.length]), passBuf]));
        stage = 1;
        return;
      }
      if (stage === 1) {
        if (buffer.length < 2) return;
        if (buffer[1] !== 0x00) return fail(`auth failed code ${buffer[1]}`);
        buffer = buffer.subarray(2);
        const hostBuf = Buffer.from(targetHost, "utf8");
        socket.write(Buffer.concat([
          Buffer.from([0x05, 0x01, 0x00, 0x03, hostBuf.length]),
          hostBuf,
          Buffer.from([(targetPort >> 8) & 0xff, targetPort & 0xff]),
        ]));
        stage = 2;
        return;
      }
      if (stage === 2) {
        if (buffer.length < 2) return;
        if (buffer[1] !== 0x00) return fail(`connect failed code ${buffer[1]}`);
        stage = 3;
        socket.write("GET / HTTP/1.1\r\nHost: api.ipify.org\r\nConnection: close\r\n\r\n");
        return;
      }
      if (stage === 3) {
        const text = buffer.toString("utf8");
        const index = text.indexOf("\r\n\r\n");
        if (index < 0) return;
        const body = text.slice(index + 4).trim();
        socket.destroy();
        resolve({ ok: true, status: text.slice(0, 20).trim(), ip: body.split("\n").pop().trim() });
      }
    });
  });
}

const suffix = Date.now().toString(36);
const variants = [
  ["plain", baseUser],
  ["session", `${baseUser}-session-${suffix}`],
  ["sessid", `${baseUser}-sessid-${suffix}`],
  ["underscore", `${baseUser}_session-${suffix}`],
];

for (const [label, user] of variants) {
  const first = await socksConnect(user, "api.ipify.org", 80);
  const second = await socksConnect(user, "api.ipify.org", 80);
  console.log(JSON.stringify({ label, user: user.replace(baseUser, "USER"), first, second, sticky: first.ok && second.ok && first.ip === second.ip }));
}
