import tls from "node:tls";

const host = process.env.IMAP_HOST || "imap.example.com";
const port = Number(process.env.IMAP_PORT || 993);
const user = process.env.IMAP_USER;
const password = process.env.IMAP_PASSWORD;
const mailbox = process.env.IMAP_MAILBOX || "INBOX";
const subject = process.env.MAIL_SUBJECT || "Your MiniMax Verification Code";
const recipient = process.env.MAIL_TO || "";
const sinceSeconds = Number(process.env.MAIL_SINCE_SECONDS || 600);
const timeoutMs = Number(process.env.IMAP_TIMEOUT_MS || 120000);

if (!user || !password) throw new Error("IMAP_USER and IMAP_PASSWORD are required");

function quoted(value) {
  return `"${String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

class ImapClient {
  constructor() {
    this.buffer = "";
    this.tag = 0;
    this.pending = new Map();
    this.socket = tls.connect({ host, port, servername: host, rejectUnauthorized: true });
    this.socket.setEncoding("binary");
    this.socket.on("data", (chunk) => this.onData(chunk));
    this.socket.on("error", (error) => {
      for (const { reject } of this.pending.values()) reject(error);
      this.pending.clear();
    });
  }

  onData(chunk) {
    this.buffer += chunk;
    while (true) {
      const lineEnd = this.buffer.indexOf("\r\n");
      if (lineEnd < 0) return;
      const line = this.buffer.slice(0, lineEnd);
      this.buffer = this.buffer.slice(lineEnd + 2);
      const pendingTag = line.match(/^A(\d+) (OK|NO|BAD)\b/i);
      if (pendingTag) {
        const id = `A${pendingTag[1]}`;
        const pending = this.pending.get(id);
        if (!pending) continue;
        this.pending.delete(id);
        if (pendingTag[2].toUpperCase() === "OK") pending.resolve({ lines: pending.lines, final: line });
        else pending.reject(new Error(line));
        continue;
      }
      const literal = line.match(/\* \d+ FETCH .*\{(\d+)\}$/);
      if (literal) {
        const length = Number(literal[1]);
        while (this.buffer.length < length + 2) return;
        const literalData = this.buffer.slice(0, length);
        this.buffer = this.buffer.slice(length + 2);
        for (const pending of this.pending.values()) pending.lines.push(line, literalData);
        continue;
      }
      for (const pending of this.pending.values()) pending.lines.push(line);
    }
  }

  async send(command, arg) {
    const id = `A${++this.tag}`;
    const line = arg === undefined ? `${id} ${command}` : `${id} ${command} ${arg}`;
    const promise = new Promise((resolve, reject) => this.pending.set(id, { resolve, reject, lines: [] }));
    this.socket.write(`${line}\r\n`, "latin1");
    return promise;
  }

  close() {
    this.socket.end();
  }
}

function decodeQuotedPrintable(value) {
  return value.replace(/=\r?\n/g, "").replace(/=([0-9A-F]{2})/gi, (_, hex) => String.fromCharCode(parseInt(hex, 16)));
}

function parseMessages(lines) {
  const combined = lines.join("\r\n");
  const messages = [];
  const starts = [...combined.matchAll(/\* (\d+) FETCH \(/g)].map((match) => ({ id: Number(match[1]), index: match.index }));
  for (let index = 0; index < starts.length; index += 1) {
    const { id } = starts[index];
    const start = starts[index].index;
    const end = starts[index + 1]?.index ?? combined.length;
    const raw = combined.slice(start, end);
    const subjectMatch = raw.match(/SUBJECT:\s?([^\r\n]+)/i);
    const dateMatch = raw.match(/DATE:\s?([^\r\n]+)/i);
    const toMatch = raw.match(/TO:\s?([^\r\n]+)/i);
    const decoded = decodeQuotedPrintable(raw);
    const codes = [...decoded.matchAll(/Your verification code is:\s*(\d{6})\b/gi)].map((codeMatch) => codeMatch[1]);
    messages.push({
      id,
      subject: subjectMatch?.[1] || "",
      date: dateMatch?.[1] || "",
      to: toMatch?.[1] || "",
      codes,
      text: decoded.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").slice(0, 1000),
    });
  }
  return messages;
}

const client = new ImapClient();
const timer = setTimeout(() => { client.close(); throw new Error("IMAP timeout"); }, timeoutMs);
timer.unref?.();

try {
  await new Promise((resolve, reject) => {
    client.socket.once("secureConnect", resolve);
    client.socket.once("error", reject);
  });
  await client.send("LOGIN", `${quoted(user)} ${quoted(password)}`);
  await client.send("SELECT", quoted(mailbox));
  const since = new Date(Date.now() - sinceSeconds * 1000);
  const day = String(since.getUTCDate()).padStart(2, "0");
  const monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  const imapDate = `${day}-${monthNames[since.getUTCMonth()]}-${since.getUTCFullYear()}`;
  const criteria = [`SINCE ${imapDate}`, `SUBJECT ${quoted(subject)}`];
  if (recipient) criteria.push(`TO ${quoted(recipient)}`);
  const search = await client.send("SEARCH", criteria.join(" "));
  const searchLine = search.lines.find((line) => /^\* SEARCH\b/i.test(line)) || "";
  const ids = searchLine.replace(/^\* SEARCH\s*/i, "").trim().split(/\s+/).filter(Boolean).map(Number);
  if (!ids.length) {
    console.log(JSON.stringify({ ok: false, reason: "no matching messages", ids: [] }, null, 2));
  } else {
    const recentIds = ids.slice(-10).join(",");
    const fetch = await client.send("FETCH", `${recentIds} (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM TO DATE)] BODY.PEEK[TEXT]<0.8192>)`);
    const messages = parseMessages(fetch.lines).filter((message) => !recipient || message.to.toLowerCase().includes(recipient.toLowerCase()));
    const latest = messages.reverse().find((message) => message.codes.length);
    console.log(JSON.stringify({ ok: !!latest, ids, code: latest?.codes.at(-1) || null, message: latest || null, messages }, null, 2));
  }
} finally {
  clearTimeout(timer);
  client.close();
}
