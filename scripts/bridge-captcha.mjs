import fs from "node:fs/promises";
import path from "node:path";

const payload = process.argv[2];
const statePath = process.env.STATE_PATH || path.join(process.cwd(), "artifacts", "minimax-session.json");

if (!payload) {
  console.error("Usage: node scripts/bridge-captcha.mjs '<JSON or key=value>'");
  process.exit(2);
}

let ticket;
let randStr = "";
let captchaType = "recaptcha";
let action = "send_code";
try {
  const input = JSON.parse(payload);
  ticket = input.ticket || input.CAPTCHA_TICKET;
  randStr = input.randStr || input.CAPTCHA_RAND_STR || "";
  captchaType = input.captchaType || input.CAPTCHA_TYPE || captchaType;
  action = input.action || input.CAPTCHA_ACTION || action;
} catch {
  const values = Object.fromEntries(payload.split("&").map((pair) => {
    const separator = pair.indexOf("=");
    return [decodeURIComponent(pair.slice(0, separator)), decodeURIComponent(pair.slice(separator + 1))];
  }));
  ticket = values.ticket || values.CAPTCHA_TICKET;
  randStr = values.randStr || values.CAPTCHA_RAND_STR || "";
  captchaType = values.captchaType || values.CAPTCHA_TYPE || captchaType;
  action = values.action || values.CAPTCHA_ACTION || action;
}

if (!ticket) throw new Error("ticket is required");

let state = { cookies: {}, localStorage: {} };
try { state = JSON.parse(await fs.readFile(statePath, "utf8")); } catch {}
state.captcha = { ticket, randStr, captchaType, action, bridgedAt: new Date().toISOString() };
await fs.mkdir(path.dirname(statePath), { recursive: true });
await fs.writeFile(statePath, JSON.stringify(state, null, 2), "utf8");
console.log(JSON.stringify({ ok: true, statePath, captchaType, action, ticketLength: ticket.length, randStrLength: randStr.length }, null, 2));
