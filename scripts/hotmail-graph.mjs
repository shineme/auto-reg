import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const command = process.argv[2] || process.env.HOTMAIL_COMMAND || "poll-code";
const subjectNeedle = process.env.MAIL_SUBJECT || "Your MiniMax Verification Code";
const recipient = (process.env.MAIL_TO || "").trim().toLowerCase();
const sinceSeconds = Number(process.env.MAIL_SINCE_SECONDS || 600);
const timeoutMs = Number(process.env.HOTMAIL_TIMEOUT_MS || process.env.IMAP_TIMEOUT_MS || 120000);
const graphScope = process.env.HOTMAIL_GRAPH_SCOPE || "https://graph.microsoft.com/Mail.ReadWrite offline_access";
const tokenEndpoint = process.env.HOTMAIL_TOKEN_URL || "https://login.microsoftonline.com/consumers/oauth2/v2.0/token";
const graphBase = process.env.HOTMAIL_GRAPH_BASE || "https://graph.microsoft.com/v1.0";

function decodeQuotedPrintable(value) {
  return String(value || "")
    .replace(/=\r?\n/g, "")
    .replace(/=([0-9A-F]{2})/gi, (_, hex) => String.fromCharCode(parseInt(hex, 16)));
}

function stripHtml(value) {
  return decodeQuotedPrintable(value)
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function extractCodes(text) {
  const source = String(text || "");
  const matches = [
    ...source.matchAll(/Your verification code is:\s*(\d{6})\b/gi),
    ...source.matchAll(/verification code[:\s]*(\d{6})\b/gi),
    ...source.matchAll(/(?:code|验证码)[^\d]{0,12}(\d{6})\b/gi),
  ];
  return [...new Set(matches.map((match) => match[1]))];
}

function isLikelyClientId(value) {
  const text = String(value || "").trim();
  if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(text)) return true;
  if (text.includes(".") && !/\s/.test(text)) return false;
  return text.length >= 8 && !text.includes(".");
}

function parseHotmailLine(line, lineNumber = 1) {
  const trimmed = String(line || "").trim();
  if (!trimmed || trimmed.startsWith("#")) return null;
  const separator = trimmed.includes("\t") ? "\t" : "----";
  const parts = separator === "\t"
    ? trimmed.split(/\t+/).map((part) => part.trim()).filter(Boolean)
    : trimmed.split(separator).map((part) => part.trim());
  if (parts.length < 4) {
    throw new Error(`第 ${lineNumber} 行格式无效，需要 email----password----clientId----refreshToken`);
  }
  const email = parts[0];
  const password = parts[1];
  const clientId = parts[2];
  const refreshToken = parts.slice(3).join(separator);
  if (!email.includes("@") || !password || !clientId || !refreshToken) {
    throw new Error(`第 ${lineNumber} 行字段不完整`);
  }
  if (!isLikelyClientId(clientId)) {
    throw new Error(`第 ${lineNumber} 行 Client ID 看起来不像微软应用 ID`);
  }
  return {
    email: email.toLowerCase(),
    password,
    clientId,
    refreshToken,
    accountType: "outlook",
    provider: "hotmail-graph",
  };
}

function parseHotmailRaw(raw) {
  const text = String(raw || "").replace(/^\ufeff/, "").replace(/\r\n/g, "\n");
  const accounts = [];
  const errors = [];
  const lines = text.split("\n");
  for (let index = 0; index < lines.length; index += 1) {
    try {
      const account = parseHotmailLine(lines[index], index + 1);
      if (account) accounts.push(account);
    } catch (error) {
      errors.push({ line: index + 1, message: error.message });
    }
  }
  const unique = new Map();
  for (const account of accounts) unique.set(account.email, account);
  return { accounts: [...unique.values()], errors };
}

async function readJson(filePath, fallback = null) {
  try {
    const raw = await fs.readFile(filePath, "utf8");
    return JSON.parse(raw.replace(/^\uFEFF/, ""));
  } catch (error) {
    if (fallback !== null && (error.code === "ENOENT" || error instanceof SyntaxError)) return fallback;
    throw error;
  }
}

async function writeJson(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  const tempPath = `${filePath}.${process.pid}.${Date.now()}.tmp`;
  await fs.writeFile(tempPath, `${JSON.stringify(value, null, 2)}\n`, "utf8");
  try {
    await fs.rename(tempPath, filePath);
  } catch (error) {
    await fs.rm(tempPath, { force: true }).catch(() => {});
    throw error;
  }
}

function poolPathFromEnv() {
  return process.env.HOTMAIL_POOL_PATH
    || path.join(process.env.BATCH_ROOT || path.join(workspace, "artifacts", "gui-batch"), "hotmail-pool.json");
}

async function withPoolLock(task, { timeoutMs = 90000, staleMs = 120000 } = {}) {
  const lockPath = `${poolPathFromEnv()}.lock`;
  const deadline = Date.now() + timeoutMs;
  let handle = null;
  while (!handle) {
    try {
      handle = await fs.open(lockPath, "wx");
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      const stat = await fs.stat(lockPath).catch(() => null);
      if (!stat || Date.now() - stat.mtimeMs > staleMs) {
        await fs.rm(lockPath, { force: true }).catch(() => {});
        continue;
      }
      if (Date.now() > deadline) throw new Error("号池文件锁等待超时，可能有残留进程未退出");
      await new Promise((resolve) => setTimeout(resolve, 30 + Math.floor(Math.random() * 60)));
    }
  }
  try {
    await handle.writeFile(`${process.pid}\n${new Date().toISOString()}\n`, "utf8");
    return await task();
  } finally {
    await handle.close().catch(() => {});
    await fs.rm(lockPath, { force: true }).catch(() => {});
  }
}

async function loadPool(filePath = poolPathFromEnv()) {
  const pool = await readJson(filePath, {
    version: 1,
    updatedAt: null,
    accounts: [],
  });
  pool.accounts = Array.isArray(pool.accounts) ? pool.accounts : [];
  return { filePath, pool };
}

function serializeAccount(account) {
  return {
    id: account.id || crypto.createHash("sha1").update(String(account.email).toLowerCase()).digest("hex").slice(0, 12),
    email: String(account.email || "").toLowerCase(),
    password: account.password || "",
    clientId: account.clientId || "",
    refreshToken: account.refreshToken || "",
    status: account.status || "available",
    lastError: account.lastError || "",
    lastCheckedAt: account.lastCheckedAt || null,
    lastUsedAt: account.lastUsedAt || null,
    usedBy: account.usedBy || null,
    importedAt: account.importedAt || new Date().toISOString(),
  };
}

async function savePool(filePath, pool) {
  pool.updatedAt = new Date().toISOString();
  await writeJson(filePath, pool);
  return pool;
}

async function importAccounts(raw, { replace = false } = {}) {
  const parsed = parseHotmailRaw(raw);
  const { filePath, pool } = await loadPool();
  const existing = new Map(pool.accounts.map((account) => [account.email.toLowerCase(), account]));
  let imported = 0;
  let updated = 0;
  for (const account of parsed.accounts) {
    const previous = existing.get(account.email);
    const preservedStatus = previous?.status;
    const nextStatus = preservedStatus === "used"
      || preservedStatus === "skipped"
      || preservedStatus === "error"
      || preservedStatus === "reserved"
      ? preservedStatus
      : "available";
    const next = serializeAccount({
      ...previous,
      ...account,
      status: nextStatus,
      importedAt: previous?.importedAt || new Date().toISOString(),
    });
    if (existing.has(account.email)) updated += 1;
    else imported += 1;
    existing.set(account.email, next);
  }
  pool.accounts = replace
    ? parsed.accounts.map((account) => serializeAccount(account))
    : [...existing.values()];
  await savePool(filePath, pool);
  return {
    ok: parsed.errors.length === 0,
    filePath,
    imported,
    updated,
    total: pool.accounts.length,
    available: pool.accounts.filter((account) => account.status === "available").length,
    used: pool.accounts.filter((account) => account.status === "used").length,
    errors: parsed.errors,
  };
}

async function refreshAccessToken(account) {
  const body = new URLSearchParams({
    client_id: account.clientId,
    grant_type: "refresh_token",
    refresh_token: account.refreshToken,
    scope: graphScope,
  });
  const response = await fetch(tokenEndpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok || !payload.access_token) {
    const detail = payload.error_description || payload.error || `HTTP ${response.status}`;
    throw new Error(`微软令牌刷新失败：${detail}`);
  }
  if (payload.refresh_token && payload.refresh_token !== account.refreshToken) {
    account.refreshToken = payload.refresh_token;
  }
  return {
    accessToken: payload.access_token,
    scope: payload.scope || "",
    refreshToken: account.refreshToken,
  };
}

async function graphRequest(accessToken, resource, init = {}) {
  const url = resource.startsWith("https://") ? resource : `${graphBase}/me/${resource.replace(/^\//, "")}`;
  const response = await fetch(url, {
    ...init,
    headers: {
      Authorization: `Bearer ${accessToken}`,
      Accept: "application/json",
      Prefer: 'outlook.body-content-type="text"',
      ...(init.headers || {}),
    },
  });
  const text = await response.text();
  let payload = null;
  try { payload = text ? JSON.parse(text) : null; } catch { payload = { raw: text }; }
  if (!response.ok) {
    const detail = payload?.error?.message || payload?.error?.code || `HTTP ${response.status}`;
    throw new Error(`Microsoft Graph 请求失败：${detail}`);
  }
  return payload;
}

function messageText(message) {
  const body = message?.body?.content || "";
  const preview = message?.bodyPreview || "";
  return stripHtml(`${preview}\n${body}`);
}

function messageRecipients(message) {
  return (message?.toRecipients || [])
    .map((item) => item?.emailAddress?.address || "")
    .filter(Boolean)
    .join(", ");
}

async function listRecentMessages(account, { top = 15 } = {}) {
  const token = await refreshAccessToken(account);
  const folders = ["inbox", "junkemail"];
  const collections = [];
  for (const folder of folders) {
    const params = new URLSearchParams({
      "$top": String(top),
      "$orderby": "receivedDateTime desc",
      "$select": "id,subject,from,toRecipients,receivedDateTime,bodyPreview,body",
    });
    const collection = await graphRequest(token.accessToken, `mailFolders/${folder}/messages?${params}`);
    collections.push(...(collection.value || []).map((message) => ({ ...message, __folder: folder })));
  }
  const since = Date.now() - sinceSeconds * 1000;
  const messages = collections.map((message) => {
    const text = messageText(message);
    const codes = extractCodes(text);
    return {
      id: message.id,
      folder: message.__folder || "inbox",
      subject: message.subject || "",
      date: message.receivedDateTime || "",
      to: messageRecipients(message),
      codes,
      text: text.slice(0, 1000),
    };
  }).filter((message) => {
    if (recipient && !message.to.toLowerCase().includes(recipient)) return false;
    if (!message.date) return true;
    const stamp = Date.parse(message.date);
    return Number.isNaN(stamp) || stamp >= since;
  });
  return { token, messages };
}

async function resolveAccount() {
  if (process.env.HOTMAIL_EMAIL && process.env.HOTMAIL_CLIENT_ID && process.env.HOTMAIL_REFRESH_TOKEN) {
    return {
      email: process.env.HOTMAIL_EMAIL.toLowerCase(),
      password: process.env.HOTMAIL_PASSWORD || "",
      clientId: process.env.HOTMAIL_CLIENT_ID,
      refreshToken: process.env.HOTMAIL_REFRESH_TOKEN,
    };
  }
  if (process.env.HOTMAIL_ACCOUNT_JSON) {
    return JSON.parse(process.env.HOTMAIL_ACCOUNT_JSON);
  }
  if (process.env.HOTMAIL_ACCOUNT_PATH) {
    return readJson(process.env.HOTMAIL_ACCOUNT_PATH);
  }
  const raw = process.env.HOTMAIL_RAW || "";
  if (raw.trim()) {
    const parsed = parseHotmailRaw(raw);
    if (!parsed.accounts.length) throw new Error(parsed.errors[0]?.message || "没有可解析的 Hotmail 账号");
    return parsed.accounts[0];
  }
  throw new Error("缺少 Hotmail Graph 账号：请设置 HOTMAIL_ACCOUNT_PATH / HOTMAIL_RAW / HOTMAIL_EMAIL");
}

async function persistRotatedToken(account) {
  if (!account?.email || !account?.refreshToken) return;
  await withPoolLock(async () => {
    const { filePath, pool } = await loadPool();
    let changed = false;
    pool.accounts = pool.accounts.map((item) => {
      if (item.email.toLowerCase() !== account.email.toLowerCase()) return item;
      if (item.refreshToken === account.refreshToken) return item;
      changed = true;
      return { ...item, refreshToken: account.refreshToken, lastCheckedAt: new Date().toISOString() };
    });
    if (changed) await savePool(filePath, pool);
  });
}

async function testAccount(account) {
  const startedAt = new Date().toISOString();
  const { token, messages } = await listRecentMessages(account, { top: 5 });
  await persistRotatedToken({ ...account, refreshToken: token.refreshToken });
  return {
    ok: true,
    email: account.email,
    scope: token.scope,
    messageCount: messages.length,
    latestSubject: messages[0]?.subject || null,
    checkedAt: startedAt,
  };
}

async function pollCode(account) {
  const { token, messages } = await listRecentMessages(account, { top: 20 });
  await persistRotatedToken({ ...account, refreshToken: token.refreshToken });
  const matched = messages
    .filter((message) => {
      if (!subjectNeedle) return true;
      const subject = message.subject.toLowerCase();
      return subject.includes(subjectNeedle.toLowerCase())
        || subject.includes("minimax")
        || subject.includes("verification")
        || message.codes.length > 0;
    })
    .find((message) => message.codes.length);
  if (matched) {
    return {
      ok: true,
      code: matched.codes.at(-1),
      message: matched,
      messages,
      email: account.email,
      transport: "hotmail-graph",
    };
  }
  return {
    ok: false,
    reason: "no matching messages",
    code: null,
    messages,
    email: account.email,
    transport: "hotmail-graph",
  };
}

async function poolStatus() {
  const { filePath, pool } = await loadPool();
  return {
    ok: true,
    filePath,
    total: pool.accounts.length,
    available: pool.accounts.filter((account) => account.status === "available").length,
    reserved: pool.accounts.filter((account) => account.status === "reserved").length,
    used: pool.accounts.filter((account) => account.status === "used").length,
    error: pool.accounts.filter((account) => account.status === "error").length,
    skipped: pool.accounts.filter((account) => account.status === "skipped").length,
    updatedAt: pool.updatedAt,
  };
}

async function claimAccount({ usedBy = "", mark = "reserved" } = {}) {
  const { filePath, pool } = await loadPool();
  const availableIndexes = pool.accounts
    .map((account, index) => ({ account, index }))
    .filter(({ account }) => account.status === "available");
  if (!availableIndexes.length) throw new Error("Hotmail 账号池已用尽，请先导入可用 Graph 账号");
  // Prefer least-recently-used accounts so soft-released failures are not immediately reclaimed.
  const cooldownMs = Math.max(0, Number(process.env.HOTMAIL_CLAIM_COOLDOWN_MS || 120000));
  const now = Date.now();
  const cooled = availableIndexes.filter(({ account }) => {
    if (!cooldownMs) return true;
    const lastUsed = Date.parse(account.lastUsedAt || "") || 0;
    if (!lastUsed) return true;
    return (now - lastUsed) >= cooldownMs;
  });
  const candidates = cooled.length ? cooled : availableIndexes;
  candidates.sort((left, right) => {
    const leftFail = Number(left.account.failCount || 0);
    const rightFail = Number(right.account.failCount || 0);
    if (leftFail !== rightFail) return leftFail - rightFail;
    const leftUsed = Date.parse(left.account.lastUsedAt || left.account.lastCheckedAt || 0) || 0;
    const rightUsed = Date.parse(right.account.lastUsedAt || right.account.lastCheckedAt || 0) || 0;
    if (leftUsed !== rightUsed) return leftUsed - rightUsed;
    return left.index - right.index;
  });
  const index = candidates[0].index;
  const claimedAt = new Date().toISOString();
  const account = {
    ...pool.accounts[index],
    status: mark === "used" ? "used" : "reserved",
    usedBy: usedBy || null,
    lastUsedAt: claimedAt,
    lastError: "",
  };
  pool.accounts[index] = account;
  await savePool(filePath, pool);
  const accountPath = process.env.HOTMAIL_ACCOUNT_PATH
    || path.join(path.dirname(filePath), "claimed", `${account.id || account.email}.json`);
  await writeJson(accountPath, account);
  return {
    ok: true,
    filePath,
    accountPath,
    email: account.email,
    account,
    available: pool.accounts.filter((item) => item.status === "available").length,
    total: pool.accounts.length,
  };
}


function isHardTokenError(message) {
  return /AADSTS70000|AADSTS50173|AADSTS700082|AADSTS50034|invalid_grant|grant is expired|微软令牌刷新失败|user could not be authenticated|mailbox.*(not found|does not exist)|account.*(disabled|locked)|interaction_required/i.test(String(message || ""));
}

async function releaseReservedAccounts({ olderThanMs = 0 } = {}) {
  const { filePath, pool } = await loadPool();
  const now = Date.now();
  let released = 0;
  const releasedEmails = [];
  const kept = [];
  const releasedAccounts = [];
  for (const account of pool.accounts) {
    if (account.status !== "reserved") {
      kept.push(account);
      continue;
    }
    const stamp = Date.parse(account.lastUsedAt || account.claimedAt || "") || 0;
    if (olderThanMs > 0 && stamp && (now - stamp) < olderThanMs) {
      kept.push(account);
      continue;
    }
    released += 1;
    releasedEmails.push(account.email);
    releasedAccounts.push({
      ...account,
      status: "available",
      failCount: Number(account.failCount || 0) + 1,
      usedBy: null,
      lastCheckedAt: new Date().toISOString(),
    });
  }
  pool.accounts = kept.concat(releasedAccounts);
  if (released) await savePool(filePath, pool);
  return {
    ok: true,
    released,
    emails: releasedEmails,
    available: pool.accounts.filter((item) => item.status === "available").length,
    reserved: pool.accounts.filter((item) => item.status === "reserved").length,
    total: pool.accounts.length,
  };
}

async function preflightPool() {
  const limit = Math.max(0, Number(process.env.HOTMAIL_PREFLIGHT_LIMIT || 0));
  const releaseReserved = process.env.HOTMAIL_RELEASE_RESERVED !== "0";
  const olderThanMs = Number(process.env.HOTMAIL_RESERVED_MAX_AGE_MS || 0);
  const releaseResult = releaseReserved
    ? await releaseReservedAccounts({ olderThanMs })
    : { ok: true, released: 0, emails: [], skipped: true };

  let { filePath, pool } = await loadPool();
  // Soft failures previously marked error can be recycled when the Graph token is still usable.
  let recycledSoftErrors = 0;
  pool.accounts = pool.accounts.map((account) => {
    if (account.status !== "error") return account;
    if (isHardTokenError(account.lastError || "")) return account;
    recycledSoftErrors += 1;
    return {
      ...account,
      status: "available",
      lastError: "",
      usedBy: null,
      lastCheckedAt: new Date().toISOString(),
    };
  });
  if (recycledSoftErrors) await savePool(filePath, pool);
  ({ filePath, pool } = await loadPool());
  const candidates = pool.accounts.filter((account) => account.status === "available");
  const selected = limit > 0 ? candidates.slice(0, limit) : candidates;
  const workers = Math.max(1, Math.min(32, Number(process.env.HOTMAIL_PREFLIGHT_CONCURRENCY || process.env.HOTMAIL_PREFLIGHT_WORKERS || 8) || 8));
  const checked = new Array(selected.length);
  let nextIndex = 0;
  let completed = 0;
  let okCount = 0;
  let errorCount = 0;
  let inFlight = 0;

  const emitProgress = (extra = {}) => {
    console.error(JSON.stringify({
      type: "hotmail-progress",
      phase: extra.phase || "preflight",
      current: completed,
      total: selected.length,
      email: extra.email || "",
      ok: extra.ok,
      hard: extra.hard,
      okCount,
      errorCount,
      workers,
      inFlight,
      message: extra.message || `测活中 ${completed}/${selected.length} · 并行 ${workers}`,
      at: new Date().toISOString(),
    }));
  };

  emitProgress({
    phase: "preflight-start",
    message: selected.length
      ? `开始测活 ${selected.length} 个账号 · 并行 ${workers}`
      : "没有可测活的可用账号",
  });

  async function worker() {
    while (true) {
      const index = nextIndex;
      nextIndex += 1;
      if (index >= selected.length) return;
      const account = selected[index];
      inFlight += 1;
      emitProgress({
        phase: "preflight",
        email: account.email,
        message: `测活中 ${completed}/${selected.length} · 并行 ${workers} · ${account.email}`,
      });
      try {
        const result = await testAccount(account);
        okCount += 1;
        checked[index] = {
          email: account.email,
          ok: true,
          messageCount: result.messageCount,
          latestSubject: result.latestSubject || null,
        };
        completed += 1;
        inFlight = Math.max(0, inFlight - 1);
        emitProgress({
          phase: "preflight-item",
          email: account.email,
          ok: true,
          message: `可用 ${account.email} · ${completed}/${selected.length}`,
        });
      } catch (error) {
        const message = error.message || String(error);
        const hard = isHardTokenError(message);
        errorCount += 1;
        checked[index] = {
          email: account.email,
          ok: false,
          hard,
          error: message.slice(0, 300),
        };
        completed += 1;
        inFlight = Math.max(0, inFlight - 1);
        emitProgress({
          phase: "preflight-item",
          email: account.email,
          ok: false,
          hard,
          message: `${hard ? "失效" : "异常"} ${account.email} · ${completed}/${selected.length}`,
        });
      }
    }
  }

  const workerCount = Math.min(workers, Math.max(1, selected.length));
  if (selected.length) {
    await Promise.all(Array.from({ length: workerCount }, () => worker()));
  }

  const checkedResults = checked.filter(Boolean);
  const byEmail = new Map(checkedResults.map((item) => [item.email.toLowerCase(), item]));
  pool.accounts = pool.accounts.map((account) => {
    const result = byEmail.get(String(account.email || "").toLowerCase());
    if (!result) return account;
    if (result.ok) {
      return {
        ...account,
        lastCheckedAt: new Date().toISOString(),
        lastError: "",
      };
    }
    return {
      ...account,
      status: result.hard ? "error" : account.status,
      lastError: result.error || "test failed",
      lastCheckedAt: new Date().toISOString(),
    };
  });
  await savePool(filePath, pool);
  return {
    ok: errorCount === 0,
    filePath,
    releasedReserved: releaseResult.released || 0,
    releasedEmails: releaseResult.emails || [],
    recycledSoftErrors,
    checked: checkedResults.length,
    workers,
    okCount,
    errorCount,
    available: pool.accounts.filter((item) => item.status === "available").length,
    reserved: pool.accounts.filter((item) => item.status === "reserved").length,
    used: pool.accounts.filter((item) => item.status === "used").length,
    error: pool.accounts.filter((item) => item.status === "error").length,
    total: pool.accounts.length,
    results: checkedResults,
  };
}

async function purgeInvalidAccounts() {
  const { filePath, pool } = await loadPool();
  const before = pool.accounts.length;
  const removed = pool.accounts.filter((account) => account.status === "error" || account.status === "skipped");
  pool.accounts = pool.accounts.filter((account) => account.status !== "error" && account.status !== "skipped");
  await savePool(filePath, pool);
  return {
    ok: true,
    filePath,
    removed: removed.length,
    removedEmails: removed.map((account) => account.email),
    before,
    total: pool.accounts.length,
    available: pool.accounts.filter((item) => item.status === "available").length,
    reserved: pool.accounts.filter((item) => item.status === "reserved").length,
    used: pool.accounts.filter((item) => item.status === "used").length,
    error: pool.accounts.filter((item) => item.status === "error").length,
    skipped: pool.accounts.filter((item) => item.status === "skipped").length,
  };
}

async function keepValidAccounts() {
  const { filePath, pool } = await loadPool();
  const before = pool.accounts.length;
  const keptStatuses = new Set(["available", "used"]);
  const removed = pool.accounts.filter((account) => !keptStatuses.has(account.status));
  pool.accounts = pool.accounts.filter((account) => keptStatuses.has(account.status));
  await savePool(filePath, pool);
  return {
    ok: true,
    filePath,
    removed: removed.length,
    removedEmails: removed.map((account) => account.email),
    before,
    total: pool.accounts.length,
    available: pool.accounts.filter((item) => item.status === "available").length,
    reserved: pool.accounts.filter((item) => item.status === "reserved").length,
    used: pool.accounts.filter((item) => item.status === "used").length,
    error: pool.accounts.filter((item) => item.status === "error").length,
  };
}

async function markAccount(status, { email = "", error = "", usedBy = "" } = {}) {
  const targetEmail = String(email || process.env.HOTMAIL_EMAIL || "").toLowerCase();
  if (!targetEmail) throw new Error("标记 Hotmail 账号时需要 email");
  const { filePath, pool } = await loadPool();
  let found = false;
  let finalStatus = status;
  let ignored = false;
  let movedToEnd = null;
  const maxSoftFails = Math.max(1, Number(process.env.HOTMAIL_SOFT_FAIL_LIMIT || 3));
  pool.accounts = pool.accounts.map((account) => {
    if (account.email.toLowerCase() !== targetEmail) return account;
    found = true;
    // Never downgrade terminal pool states during retries / reconcile.
    if (account.status === "used" && status !== "used") {
      ignored = true;
      finalStatus = account.status;
      return account;
    }
    if (account.status === "skipped" && (status === "available" || status === "reserved")) {
      ignored = true;
      finalStatus = account.status;
      return account;
    }
    if (account.status === "error" && (status === "available" || status === "reserved")) {
      ignored = true;
      finalStatus = account.status;
      return account;
    }
    const nextFailCount = status === "available"
      ? Number(account.failCount || 0) + 1
      : status === "used"
        ? 0
        : Number(account.failCount || 0);
    let nextStatus = status;
    // Soft-released accounts that keep failing get parked so they stop monopolizing the claim queue.
    if (status === "available" && nextFailCount >= maxSoftFails) {
      nextStatus = "skipped";
      finalStatus = nextStatus;
    } else {
      finalStatus = nextStatus;
    }
    const updated = {
      ...account,
      status: nextStatus,
      lastError: error || account.lastError || "",
      usedBy: usedBy || account.usedBy || null,
      failCount: nextFailCount,
      lastUsedAt: new Date().toISOString(),
      lastCheckedAt: new Date().toISOString(),
    };
    if (nextStatus === "available") {
      movedToEnd = updated;
      return null;
    }
    return updated;
  });
  if (!found) throw new Error(`Hotmail 账号池中找不到 ${targetEmail}`);
  if (movedToEnd) {
    pool.accounts = pool.accounts.filter(Boolean);
    pool.accounts.push(movedToEnd);
  } else {
    pool.accounts = pool.accounts.filter(Boolean);
  }
  if (!ignored) await savePool(filePath, pool);
  return {
    ok: true,
    email: targetEmail,
    status: finalStatus,
    ignored,
    available: pool.accounts.filter((item) => item.status === "available").length,
    total: pool.accounts.length,
  };
}

async function main() {
  switch (command) {
    case "parse": {
      const raw = process.env.HOTMAIL_RAW || await fs.readFile(process.env.HOTMAIL_RAW_PATH || "", "utf8").catch(() => "");
      const parsed = parseHotmailRaw(raw);
      console.log(JSON.stringify({ ok: parsed.errors.length === 0, ...parsed, count: parsed.accounts.length }, null, 2));
      break;
    }
    case "import": {
      const raw = process.env.HOTMAIL_RAW || await fs.readFile(process.env.HOTMAIL_RAW_PATH || "", "utf8");
      const result = await withPoolLock(() => importAccounts(raw, { replace: process.env.HOTMAIL_REPLACE === "1" }));
      console.log(JSON.stringify(result, null, 2));
      if (!result.ok) process.exitCode = 1;
      break;
    }
    case "status": {
      console.log(JSON.stringify(await poolStatus(), null, 2));
      break;
    }
    case "claim": {
      const claimed = await withPoolLock(() => claimAccount({
        usedBy: process.env.HOTMAIL_USED_BY || "",
        mark: process.env.HOTMAIL_CLAIM_MARK || "reserved",
      }));
      console.log(JSON.stringify(claimed, null, 2));
      break;
    }
    case "mark-used": {
      const marked = await withPoolLock(() => markAccount("used", {
        email: process.env.HOTMAIL_EMAIL || "",
        usedBy: process.env.HOTMAIL_USED_BY || "",
      }));
      console.log(JSON.stringify(marked, null, 2));
      break;
    }
    case "mark-error": {
      const marked = await withPoolLock(() => markAccount("error", {
        email: process.env.HOTMAIL_EMAIL || "",
        error: process.env.HOTMAIL_ERROR || "",
        usedBy: process.env.HOTMAIL_USED_BY || "",
      }));
      console.log(JSON.stringify(marked, null, 2));
      break;
    }
    case "mark-skipped": {
      const marked = await withPoolLock(() => markAccount("skipped", {
        email: process.env.HOTMAIL_EMAIL || "",
        error: process.env.HOTMAIL_ERROR || "",
        usedBy: process.env.HOTMAIL_USED_BY || "",
      }));
      console.log(JSON.stringify(marked, null, 2));
      break;
    }
    case "release": {
      const marked = await withPoolLock(() => markAccount("available", {
        email: process.env.HOTMAIL_EMAIL || "",
        error: process.env.HOTMAIL_ERROR || "",
      }));
      console.log(JSON.stringify(marked, null, 2));
      break;
    }
    case "purge-invalid": {
      console.log(JSON.stringify(await withPoolLock(() => purgeInvalidAccounts()), null, 2));
      break;
    }
    case "keep-valid": {
      console.log(JSON.stringify(await withPoolLock(() => keepValidAccounts()), null, 2));
      break;
    }
    case "release-reserved": {
      console.log(JSON.stringify(await withPoolLock(() => releaseReservedAccounts({
        olderThanMs: Number(process.env.HOTMAIL_RESERVED_MAX_AGE_MS || 0),
      })), null, 2));
      break;
    }
    case "preflight": {
      const result = await preflightPool();
      console.log(JSON.stringify(result, null, 2));
      if (!result.ok && process.env.HOTMAIL_PREFLIGHT_STRICT === "1") process.exitCode = 1;
      break;
    }
    case "test": {
      const account = await resolveAccount();
      console.log(JSON.stringify(await testAccount(account), null, 2));
      break;
    }
    case "poll-code": {
      const account = await resolveAccount();
      const result = await pollCode(account);
      console.log(JSON.stringify(result, null, 2));
      // Keep exit 0 on empty inbox so the outer registrar can keep polling,
      // matching scripts/imap.mjs behavior.
      break;
    }
    default:
      throw new Error(`未知命令：${command}`);
  }
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message }, null, 2));
  process.exitCode = 1;
});
