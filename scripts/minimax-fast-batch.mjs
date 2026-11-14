import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const panelBase = process.env.PANEL_BASE || "https://panel.example.com";
const panelKey = process.env.PANEL_KEY || "";
const imapUser = process.env.IMAP_USER || "";
const imapPassword = process.env.IMAP_PASSWORD || "";
const mailDomain = process.env.MAIL_DOMAIN || "example.com";
const mailPrefix = process.env.MAIL_PREFIX || "mm";
const proxyUrl = process.env.MINIMAX_PROXY_URL || "";
const cdpHost = process.env.CDP_HOST || "127.0.0.1";
const cdpPort = Number(process.env.CDP_PORT || 9333);
const chromePath = process.env.CHROME_PATH || "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const chromeProfile = process.env.CHROME_PROFILE || path.join(workspace, "artifacts", "chrome-fast-profile");
const batchRoot = process.env.BATCH_ROOT || path.join(workspace, "artifacts", "fast-batch");
const batchId = process.env.BATCH_ID || new Date().toISOString().replace(/[-:T]/g, "").slice(0, 14);
const root = path.join(batchRoot, batchId);
const summaryPath = path.join(root, "summary.json");
const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
const restartCount = Number(process.env.MINIMAX_RESTART_COUNT || 0);
const dryRun = process.env.DRY_RUN === "1";
const panelCheckOnly = process.env.PANEL_CHECK_ONLY === "1";
const prewarmConcurrency = Number(process.env.PREWARM_CONCURRENCY || 4);
const signInPollSeconds = Number(process.env.SIGN_IN_POLL_SECONDS || 90);
const captchaReadyAttempts = Number(process.env.CAPTCHA_READY_ATTEMPTS || 60);
const captchaPrewarmUrl = process.env.CAPTCHA_PREWARM_URL || "https://account.minimax.io/unified-login";
const riskControlPattern = /1200063|人机验证未通过|Verification failed, please try again/i;
const riskExitCode = 75;
const captchaUnavailablePattern = /reCAPTCHA Enterprise did not become ready|reCAPTCHA Enterprise returned no ticket|CDP browser did not become ready|CDP version HTTP|ECONNRESET|socket connection was closed unexpectedly|The operation was aborted due to timeout|fetch failed/i;
const riskProbe = process.env.MINIMAX_RISK_PROBE === "1";
const riskControlPath = path.join(root, "risk-control.json");
const siteKey = process.env.MINIMAX_RECAPTCHA_SITE_KEY || "";
const captchaAction = "send_code";
const uniqueUserIdKey = "mmx_account_UNIQUE_USER_ID";
const deviceIdKey = "mmx_account_USER_HARD_WARE_INFO";
const userDetailKey = "user_detail_agent";
const screenWidth = 3440;
const screenHeight = 1440;

const panelCookies = new Map();
const pendingCommands = new Map();
let nextCommandId = 1;
let progressState = {
  target: targetSuccessCount,
  succeeded: 0,
  failed: 0,
  attempts: 0,
  current: 0,
  restartCount,
};

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function prewarmUrl(userCode, loginRedirect) {
  const url = new URL(captchaPrewarmUrl);
  url.searchParams.set("login_redirect", loginRedirect || `/oauth-authorize?user_code=${userCode}`);
  return url.toString();
}

async function writeJson(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, JSON.stringify(value, null, 2), "utf8");
}

async function readJson(filePath) {
  return JSON.parse(await fs.readFile(filePath, "utf8"));
}

function publicResult(result) {
  return {
    ok: Boolean(result.ok),
    email: result.email || "",
    user: result.user || "",
    userCode: result.userCode || "",
    signInId: result.signInId || "",
    flowStatus: result.flowStatus || "",
    signInState: result.signInState || "",
    error: result.error || "",
    runDir: result.runDir || "",
    startedAt: result.startedAt || "",
    completedAt: result.completedAt || "",
    riskBlocked: Boolean(result.riskBlocked),
  };
}

function updateProgressState(results, current) {
  progressState.succeeded = results.filter((result) => result.ok).length;
  progressState.failed = results.filter((result) => !result.ok && !result.riskBlocked).length;
  progressState.attempts = results.filter((result) => !result.riskBlocked).length;
  progressState.current = current;
}

function emitProgress(phase, message, extra = {}) {
  const event = {
    type: "progress",
    batchId,
    phase,
    message,
    count: targetSuccessCount,
    done: progressState.succeeded,
    ok: progressState.succeeded,
    failed: progressState.failed,
    attempts: progressState.attempts,
    current: extra.current || progressState.current,
    restartCount: progressState.restartCount,
    updatedAt: new Date().toISOString(),
  };
  console.log(JSON.stringify(event));
}

function emitAccountResult(result) {
  const event = {
    type: "account-result",
    batchId,
    result: publicResult(result),
    count: targetSuccessCount,
    done: progressState.succeeded,
    ok: progressState.succeeded,
    failed: progressState.failed,
    attempts: progressState.attempts,
    current: progressState.current,
    restartCount: progressState.restartCount,
    updatedAt: new Date().toISOString(),
  };
  console.log(JSON.stringify(event));
}

function accountIndex(result) {
  const match = /account-(\d+)/.exec(result.runDir || "");
  return match ? Number(match[1]) : 0;
}

async function loadExistingResults() {
  const entries = await fs.readdir(root, { withFileTypes: true }).catch(() => []);
  const results = [];
  let nextIndex = 1;
  for (const entry of entries) {
    if (!entry.isDirectory() || !entry.name.startsWith("account-")) continue;
    const match = /^account-(\d+)$/.exec(entry.name);
    if (match) nextIndex = Math.max(nextIndex, Number(match[1]) + 1);
    const resultPath = path.join(root, entry.name, "result.json");
    try {
      const result = JSON.parse(await fs.readFile(resultPath, "utf8"));
      if (!result.ok && (riskControlPattern.test(result.error || "") || captchaUnavailablePattern.test(result.error || ""))) {
        await fs.rm(resultPath, { force: true });
        continue;
      }
      if (typeof result.ok === "boolean") results.push(result);
    } catch {}
  }
  results.sort((left, right) => accountIndex(left) - accountIndex(right));
  return { results, nextIndex };
}

function cookieHeader() {
  return [...panelCookies.entries()].map(([name, value]) => `${name}=${value}`).join("; ");
}

function redactText(value) {
  return String(value)
    .replace(/(^|[?&])((?:k|key|token|password|secret|ticket|auth|authorization|code)=)[^&\s"'<>]+/gi, "$1$2***")
    .replace(/(PANEL_KEY|IMAP_PASSWORD|ticket|Proxy-Authorization|Authorization)([=:]\s*)[^\s,;]+/gi, "$1$2***")
    .replace(/(Bearer\s+)[A-Za-z0-9._~+/=-]+/gi, "$1***");
}

function safePath(pathname) {
  try {
    const parsed = new URL(pathname, panelBase);
    for (const key of parsed.searchParams.keys()) {
      if (/^(k|key|token|password|secret|ticket|auth|authorization|code)$/i.test(key)) parsed.searchParams.set(key, "***");
    }
    return `${parsed.pathname}${parsed.search}`;
  } catch {
    return redactText(pathname);
  }
}

function storeSetCookie(response) {
  const values = typeof response.headers.getSetCookie === "function"
    ? response.headers.getSetCookie()
    : (response.headers.get("set-cookie") ? [response.headers.get("set-cookie")] : []);
  const fallback = response.headers.get("set-cookie");
  if (!values.length && fallback) values.push(fallback);
  for (const value of values) {
    const pair = value.split(";")[0];
    const separator = pair.indexOf("=");
    if (separator > 0) panelCookies.set(pair.slice(0, separator).trim(), pair.slice(separator + 1).trim());
  }
}

async function panelFetch(pathname, options = {}, expectJson = true) {
  const requestUrl = new URL(pathname, panelBase);
  let method = options.method || "GET";
  const body = options.body;
  let redirectCount = 0;
  let currentUrl = requestUrl;
  let response;
  while (redirectCount <= 5) {
    const headers = new Headers(options.headers || {});
    if (panelCookies.size) headers.set("Cookie", cookieHeader());
    const requestOptions = { method, headers, redirect: "manual" };
    if (body !== undefined && redirectCount === 0) requestOptions.body = body;
    response = await fetch(currentUrl, requestOptions);
    storeSetCookie(response);
    const location = response.headers.get("location");
    if ([301, 302, 303, 307, 308].includes(response.status) && location) {
      currentUrl = new URL(location, currentUrl);
      redirectCount += 1;
      if (response.status === 303 || response.status === 302) method = "GET";
      continue;
    }
    break;
  }
  const text = await response.text();
  if (!response.ok) throw new Error(`${method} ${safePath(pathname)} HTTP ${response.status}: ${redactText(text.slice(0, 500))}`);
  if (!expectJson) return text;
  try {
    return JSON.parse(text);
  } catch {
    throw new Error(`${method} ${safePath(pathname)} did not return JSON: ${redactText(text.slice(0, 500))}`);
  }
}

async function cdpVersion() {
  const response = await fetch(`http://${cdpHost}:${cdpPort}/json/version`);
  if (!response.ok) throw new Error(`CDP version HTTP ${response.status}`);
  return response.json();
}

async function ensureCdpBrowser() {
  const statePath = path.join(chromeProfile, "proxy-state.json");
  try {
    const version = await cdpVersion();
    const savedState = await readJson(statePath);
    if (savedState?.proxyUrl === proxyUrl && savedState?.cdpPort === cdpPort) return version;
    try {
      const socket = new WebSocket(version.webSocketDebuggerUrl);
      await new Promise((resolve, reject) => {
        socket.onopen = resolve;
        socket.onerror = reject;
      });
      await cdpCall(socket, "Browser.close", {});
      socket.close();
    } catch {}
    for (let attempt = 0; attempt < 30; attempt += 1) {
      try {
        await cdpVersion();
      } catch {
        break;
      }
      await delay(200);
    }
  } catch {}
  await fs.mkdir(chromeProfile, { recursive: true });
  await writeJson(statePath, { proxyUrl, cdpPort, updatedAt: new Date().toISOString() });
  const args = [
    `--remote-debugging-port=${cdpPort}`,
    `--user-data-dir=${chromeProfile}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-background-networking",
    "--window-position=-32000,-32000",
    `--window-size=${screenWidth},${screenHeight}`,
  ];
  if (proxyUrl) args.push(`--proxy-server=${proxyUrl}`);
  args.push("about:blank");
  const child = spawn(chromePath, args, { detached: true, stdio: "ignore", windowsHide: true });
  child.unref();
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      return await cdpVersion();
    } catch {
      await delay(500);
    }
  }
  throw new Error(`CDP browser did not become ready on ${cdpHost}:${cdpPort}`);
}

function cdpCall(socket, method, params = {}, sessionId) {
  const id = nextCommandId++;
  return new Promise((resolve, reject) => {
    pendingCommands.set(id, { resolve, reject });
    const message = { id, method, params };
    if (sessionId) message.sessionId = sessionId;
    socket.send(JSON.stringify(message));
  });
}

async function connectCdpBrowser() {
  const version = await ensureCdpBrowser();
  const socket = new WebSocket(version.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    socket.onopen = resolve;
    socket.onerror = reject;
  });
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (!message.id || !pendingCommands.has(message.id)) return;
    const { resolve, reject } = pendingCommands.get(message.id);
    pendingCommands.delete(message.id);
    if (message.error) reject(new Error(JSON.stringify(message.error)));
    else resolve(message.result);
  };
  socket.onclose = () => {
    for (const { reject } of pendingCommands.values()) reject(new Error("CDP socket closed"));
    pendingCommands.clear();
  };
  return { socket, call: (method, params, sessionId) => cdpCall(socket, method, params, sessionId) };
}

async function evaluate(cdp, sessionId, expression, options = {}) {
  const result = await cdp.call("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: options.awaitPromise || false,
    userGesture: options.userGesture || false,
  }, sessionId);
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text || "Runtime.evaluate failed");
  return result.result.value;
}

async function disposeBrowserContext(cdp, browserContextId) {
  if (!browserContextId) return;
  try {
    await cdp.call("Target.disposeBrowserContext", { browserContextId });
  } catch {}
}

async function prewarmAccount(cdp, task) {
  let browserContextId;
  try {
    ({ browserContextId } = await cdp.call("Target.createBrowserContext", { disposeOnDetach: true }));
    const { targetId } = await cdp.call("Target.createTarget", { url: "about:blank", browserContextId });
    const { sessionId } = await cdp.call("Target.attachToTarget", { targetId, flatten: true });
    await cdp.call("Page.enable", {}, sessionId);
    await cdp.call("Runtime.enable", {}, sessionId);
    await cdp.call("Network.enable", {}, sessionId);
    await cdp.call("Emulation.setDeviceMetricsOverride", {
      width: screenWidth,
      height: screenHeight,
      deviceScaleFactor: 1,
      mobile: false,
    }, sessionId);
    await cdp.call("Page.navigate", { url: task.prewarmUrl || prewarmUrl(task.userCode, task.loginRedirect) }, sessionId);
    let ready = false;
    for (let attempt = 0; attempt < captchaReadyAttempts; attempt += 1) {
      const pageStateText = await evaluate(cdp, sessionId, `(function () {
        try {
          return JSON.stringify({
            href: location.href,
            ready: document.readyState,
            grecaptcha: typeof window.grecaptcha !== "undefined" && typeof window.grecaptcha.enterprise !== "undefined"
          });
        } catch {
          return "{}";
        }
      })()`);
      const pageState = JSON.parse(pageStateText || "{}");
      if (pageState.grecaptcha && pageState.ready === "complete") {
        ready = true;
        break;
      }
      await delay(500);
    }
    if (!ready) throw new Error("reCAPTCHA Enterprise did not become ready");
    const ticket = await evaluate(cdp, sessionId, `(async () => {
      await new Promise((resolve) => window.grecaptcha.enterprise.ready(resolve));
      return await window.grecaptcha.enterprise.execute("${siteKey}", { action: "${captchaAction}" });
    })()`, { awaitPromise: true, userGesture: true });
    if (typeof ticket !== "string" || !ticket) throw new Error("reCAPTCHA Enterprise returned no ticket");
    const cookieResult = await cdp.call("Network.getCookies", { urls: ["https://account.minimax.io/"] }, sessionId);
    const storageText = await evaluate(cdp, sessionId, `(function () {
      try {
        return JSON.stringify({
          uuid: localStorage.getItem("${uniqueUserIdKey}"),
          deviceId: localStorage.getItem("${deviceIdKey}")
        });
      } catch {
        return "{}";
      }
    })()`);
    const storage = JSON.parse(storageText || "{}");
    const state = { cookies: {}, localStorage: {}, captcha: { ticket, randStr: "", captchaType: "recaptcha", action: captchaAction, bridgedAt: new Date().toISOString() } };
    for (const cookie of cookieResult.cookies || []) state.cookies[cookie.name] = cookie.value;
    if (storage.uuid) state.localStorage[uniqueUserIdKey] = storage.uuid;
    if (storage.deviceId) state.localStorage[deviceIdKey] = storage.deviceId;
    await writeJson(task.statePath, state);
    return { browserContextId, sessionId };
  } catch (error) {
    await disposeBrowserContext(cdp, browserContextId);
    throw error;
  }
}

async function mapWithConcurrency(items, limit, worker) {
  const results = new Array(items.length);
  let nextIndex = 0;
  async function runner() {
    while (nextIndex < items.length) {
      const currentIndex = nextIndex++;
      results[currentIndex] = await worker(items[currentIndex], currentIndex);
    }
  }
  const runners = Array.from({ length: Math.min(limit, items.length) }, runner);
  await Promise.all(runners);
  return results;
}

function runNodeScript(scriptPath, env, timeoutMs = 300000, progressHandler = () => {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [scriptPath], { cwd: workspace, env, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    let stderrTail = "";
    const stderrErrors = [];
    const timer = setTimeout(() => child.kill("SIGTERM"), timeoutMs);
    child.stdout.setEncoding("utf8").on("data", (chunk) => { stdout += chunk; });
    child.stderr.setEncoding("utf8").on("data", (chunk) => {
      stderr += chunk;
      stderrTail += chunk;
      const lines = stderrTail.split(/\r?\n/);
      stderrTail = lines.pop() || "";
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        let payload = null;
        try {
          payload = JSON.parse(trimmed);
        } catch {
          stderrErrors.push(trimmed);
          continue;
        }
        if (payload?.type === "flow-progress") progressHandler(payload.step, payload.message);
        else stderrErrors.push(trimmed);
      }
    });
    child.on("error", (error) => { clearTimeout(timer); reject(error); });
    child.on("close", (code) => {
      clearTimeout(timer);
      if (stderrTail.trim()) stderrErrors.push(stderrTail.trim());
      if (code !== 0) return reject(new Error(`${path.basename(scriptPath)} exited ${code}: ${stderrErrors.join("\n") || stdout}`));
      try {
        resolve(JSON.parse(stdout));
      } catch (error) {
        reject(new Error(`${path.basename(scriptPath)} returned invalid JSON: ${stderr || stdout || error.message}`));
      }
    });
  });
}

function taskEnv(task) {
  return {
    ...process.env,
    STATE_PATH: task.statePath,
    RUN_CONFIG_PATH: task.configPath,
    FLOW_PATH: task.flowPath,
    USER_CODE: task.userCode,
    EMAIL: task.email,
    PASSWORD: task.password,
    LOGIN_REDIRECT: task.loginRedirect,
    CAPTCHA_ACTION: captchaAction,
    IMAP_USER: imapUser,
    IMAP_PASSWORD: imapPassword,
    MAIL_TO: task.email,
    MAIL_SINCE_SECONDS: "600",
    POLL_TIMEOUT_MS: "180000",
    IMAP_TIMEOUT_MS: "120000",
    MINIMAX_PROXY_URL: proxyUrl,
  };
}

async function pollSignIn(task) {
  const deadline = Date.now() + signInPollSeconds * 1000;
  let latest = null;
  while (Date.now() < deadline) {
    try {
      latest = await panelFetch(`/api/signin/${encodeURIComponent(task.signInId)}`);
      if (["done", "failed", "error"].includes(latest.state)) return latest;
    } catch (error) {
      if (!/HTTP 404\b/.test(error.message)) throw error;
      return { state: "missing", missing: true, error: "sign-in record not found" };
    }
    await delay(2000);
  }
  return latest;
}

async function providerUsers(beforeUsers = []) {
  const providers = await panelFetch("/api/providers");
  const users = new Set();
  const collect = (value) => {
    if (Array.isArray(value)) {
      for (const item of value) collect(item);
      return;
    }
    if (!value || typeof value !== "object") return;
    for (const [key, child] of Object.entries(value)) {
      if (["user", "username", "name"].includes(key) && typeof child === "string" && /^MiniMax\d+$/.test(child)) users.add(child);
      collect(child);
    }
  };
  collect(providers);
  const currentUsers = [...users];
  return { providers, users: currentUsers, newUsers: currentUsers.filter((candidate) => !beforeUsers.includes(candidate)) };
}

async function waitForProviderUser(task, beforeUsers) {
  const deadline = Date.now() + signInPollSeconds * 1000;
  let latest = null;
  while (Date.now() < deadline) {
    latest = await pollSignIn(task);
    const providerState = await providerUsers(beforeUsers);
    const user = latest?.user || providerState.newUsers[0];
    if ((latest?.state === "done" && user) || (user && providerState.users.includes(user))) {
      return { signIn: latest, providers: providerState.providers, user };
    }
    if (["failed", "error"].includes(latest?.state)) break;
    await delay(2000);
  }
  return { signIn: latest, providers: null, user: null };
}

async function createTask(index) {
    const signIn = await panelFetch("/api/plugin-signin", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ provider: "minimax-code-global", method: 0, inputs: {}, proxy: "" }),
    });
    const signInUrl = new URL(signIn.url);
    const loginRedirect = `${signInUrl.pathname}${signInUrl.search}`;
    const email = `${mailPrefix}${crypto.randomBytes(5).toString("hex")}@${mailDomain}`;
    const password = `Mm!${crypto.randomBytes(12).toString("base64url")}`;
    const runDir = path.join(root, `account-${String(index).padStart(3, "0")}`);
    return {
      index,
      signIn,
      signInId: signIn.id,
      userCode: signIn.code,
      loginRedirect,
      email,
      password,
      runDir,
      signInPath: path.join(runDir, "signin.json"),
      configPath: path.join(runDir, "run-config.json"),
      statePath: path.join(runDir, "session.json"),
      flowPath: path.join(runDir, "flow.json"),
      prewarmUrl: prewarmUrl(signIn.code, loginRedirect),
      signInVerifyPath: path.join(runDir, "signin-verify.json"),
      providersPath: path.join(runDir, "providers.json"),
      resultPath: path.join(runDir, "result.json"),
    };
}

async function initializeTask(task) {
  await fs.mkdir(task.runDir, { recursive: true });
  await fs.rm(task.statePath, { force: true });
  await writeJson(task.signInPath, task.signIn);
  await writeJson(task.configPath, { userCode: task.userCode, email: task.email, password: task.password, loginRedirect: task.loginRedirect, createdAt: new Date().toISOString() });
}

async function processAccount(cdp, task, beforeUsers, progressHandler) {
  const startedAt = new Date().toISOString();
  try {
    if (!task.context) throw new Error(task.prewarmError || "CAPTCHA context unavailable");
    progressHandler("protocol", "正在注册、接码并授权");
    await runNodeScript(path.join(workspace, "scripts", "minimax-flow.mjs"), taskEnv(task), 300000, progressHandler);
    const flow = await readJson(task.flowPath);
    if (flow.status !== "approved") throw new Error(`MiniMax flow status is ${flow.status}`);
    progressHandler("provider", "正在等待面板绑定供应商账号");
    const verification = await waitForProviderUser(task, beforeUsers);
    await writeJson(task.signInVerifyPath, verification.signIn);
    if (verification.providers) await writeJson(task.providersPath, verification.providers);
    const user = verification.user;
    if (!user) throw new Error(`magpie sign-in state is ${verification.signIn?.state || "unknown"}; provider list missing new user`);
    const result = {
      ok: true,
      email: task.email,
      password: task.password,
      user,
      userCode: task.userCode,
      signInId: task.signInId,
      flowStatus: flow.status,
      signInState: verification.signIn?.state || "verified-by-providers",
      runDir: task.runDir,
      startedAt,
      completedAt: new Date().toISOString(),
    };
    await writeJson(task.resultPath, result);
    return { index: task.index, ok: true, user: result.user, email: result.email, runDir: task.runDir, startedAt };
  } catch (error) {
    const riskBlocked = riskControlPattern.test(error.message);
    if (riskBlocked) await fs.rm(task.resultPath, { force: true });
    const result = {
      ok: false,
      email: task.email,
      password: task.password,
      userCode: task.userCode,
      signInId: task.signInId,
      error: error.message,
      riskBlocked,
      runDir: task.runDir,
      startedAt,
      completedAt: new Date().toISOString(),
    };
    if (!riskBlocked) await writeJson(task.resultPath, result);
    return { index: task.index, ok: false, error: error.message, riskBlocked, runDir: task.runDir, startedAt };
  } finally {
    if (task.context) await disposeBrowserContext(cdp, task.context.browserContextId);
  }
}

async function writeSummary(results, finished = false) {
  const counted = results.filter((result) => !result.riskBlocked);
  const succeeded = counted.filter((result) => result.ok).length;
  const failed = counted.filter((result) => !result.ok).length;
  const summary = {
    batchId,
    requested: targetSuccessCount,
    completed: succeeded,
    done: succeeded,
    succeeded,
    failed,
    attempts: counted.length,
    updatedAt: new Date().toISOString(),
    results,
  };
  if (finished || succeeded >= targetSuccessCount) summary.finishedAt = summary.updatedAt;
  await writeJson(summaryPath, summary);
  return summary;
}

async function runDryRun() {
  await fs.mkdir(root, { recursive: true });
  const cdp = await connectCdpBrowser();
  const dummyCode = process.env.DRY_RUN_USER_CODE || "TEST-CODE";
  const task = { prewarmUrl: prewarmUrl(dummyCode), statePath: path.join(root, "dryrun-session.json") };
  try {
    const context = await prewarmAccount(cdp, task);
    const state = await readJson(task.statePath);
    const result = {
      ok: true,
      ticketLength: state.captcha?.ticket?.length || 0,
      cookieNames: Object.keys(state.cookies || {}),
      uuid: state.localStorage?.[uniqueUserIdKey] || null,
      deviceId: state.localStorage?.[deviceIdKey] || null,
      statePath: task.statePath,
    };
    await writeJson(path.join(root, "dryrun.json"), result);
    console.log(JSON.stringify(result));
    await disposeBrowserContext(cdp, context.browserContextId);
  } finally {
    cdp.socket.close();
  }
}

async function runPanelCheck() {
  if (!panelKey) throw new Error("PANEL_KEY is required for panel check");
  await panelFetch(`/?k=${encodeURIComponent(panelKey)}&view=providers&edit=minimax-code-global`, {}, false);
  const providers = await panelFetch("/api/providers");
  const result = {
    ok: true,
    panelBase,
    cookies: [...panelCookies.keys()],
    providerKeys: Object.keys(providers || {}).slice(0, 20),
    hasProvidersArray: Array.isArray(providers?.providers),
    checkedAt: new Date().toISOString(),
  };
  console.log(JSON.stringify(result));
}

async function main() {
  if (!panelCheckOnly && count < 1) throw new Error("COUNT must be at least 1");
  if (!panelCheckOnly && siteKey.startsWith("<")) throw new Error("MINIMAX_RECAPTCHA_SITE_KEY must be configured locally");
  if (panelCheckOnly) {
    await runPanelCheck();
    return;
  }
  if (dryRun) {
    await runDryRun();
    return;
  }
  if (!panelKey || !imapUser || !imapPassword) throw new Error("PANEL_KEY, IMAP_USER and IMAP_PASSWORD are required");
  await fs.mkdir(root, { recursive: true });
  await panelFetch(`/?k=${encodeURIComponent(panelKey)}&view=providers&edit=minimax-code-global`, {}, false);
  const cdp = await connectCdpBrowser();
  const existing = await loadExistingResults();
  const results = existing.results;
  let nextIndex = existing.nextIndex;
  let riskBlocked = false;
  let captchaUnavailable = false;
  let riskProbeLimited = false;
  updateProgressState(results, nextIndex);
  await writeSummary(results);
  emitProgress("resume", `已恢复 ${progressState.succeeded}/${targetSuccessCount} 个成功账号，正在继续补齐`);
  try {
    while (progressState.succeeded < targetSuccessCount) {
      progressState.current = nextIndex;
      emitProgress("creating-code", `正在创建第 ${nextIndex} 个账号设备码`, { current: nextIndex });
      const task = await createTask(nextIndex);
      nextIndex += 1;
      await initializeTask(task);
      const beforeUsers = (await providerUsers()).users;
      emitProgress("captcha", `正在为 ${task.email} 预热 reCAPTCHA 票据`, { current: task.index });
      let result;
      try {
        task.context = await prewarmAccount(cdp, task);
      } catch (error) {
        task.prewarmError = error.message;
        if (!riskControlPattern.test(error.message) && !captchaUnavailablePattern.test(error.message)) {
          await writeJson(task.resultPath, { ok: false, error: task.prewarmError, email: task.email, userCode: task.userCode, signInId: task.signInId, runDir: task.runDir, completedAt: new Date().toISOString() });
        }
      }
      const reportFlowProgress = (step, message) => emitProgress(step, message, { current: task.index });
      result = await processAccount(cdp, task, beforeUsers, reportFlowProgress);
      results.push(result);
      updateProgressState(results, nextIndex);
      await writeSummary(results);
      emitAccountResult(result);
      if (result.riskBlocked) {
        riskBlocked = true;
        results.pop();
        updateProgressState(results, nextIndex);
        await writeSummary(results);
        await writeJson(riskControlPath, {
          at: new Date().toISOString(),
          error: result.error,
          email: task.email,
          userCode: task.userCode,
          signInId: task.signInId,
          runDir: task.runDir,
        });
        emitProgress("risk-cooldown", "MiniMax 人机验证未通过，本轮已停止，等待风控冷却后自动探测", { current: task.index });
        return;
      }
      if (captchaUnavailablePattern.test(result.error || "")) {
        captchaUnavailable = true;
        emitProgress("risk-cooldown", "reCAPTCHA 票据环境暂不可用，本轮已停止，等待冷却后自动探测", { current: task.index });
        return;
      }
      if (riskProbe && !result.ok) {
        riskProbeLimited = true;
        emitProgress("risk-cooldown", "单次探测未恢复注册，等待下一轮低频探测", { current: task.index });
        return;
      }
      emitProgress("account-done", result.ok ? `账号注册成功：${result.user || result.email}` : `账号未成功：${result.error || "未知错误"}`, { current: task.index });
      if (riskProbe && result.ok) {
        riskProbeLimited = true;
        emitProgress("risk-recovered", "单次探测注册成功，守护进程将恢复完整批次", { current: task.index });
        return;
      }
    }
  } finally {
    if (riskBlocked || captchaUnavailable || riskProbeLimited) {
      try {
        await cdp.call("Browser.close");
      } catch {}
    }
    cdp.socket.close();
  }
  if (riskBlocked) {
    process.exitCode = riskExitCode;
    return;
  }
  if (captchaUnavailable) {
    await writeJson(riskControlPath, {
      at: new Date().toISOString(),
      error: "reCAPTCHA 票据环境暂不可用",
      runDir: root,
      captchaUnavailable: true,
    });
    process.exitCode = riskExitCode;
    return;
  }
  if (riskProbeLimited) {
    if (riskProbe) await fs.rm(riskControlPath, { force: true });
    if (riskProbe) process.exitCode = 0;
    else process.exitCode = 1;
    return;
  }
  await fs.rm(riskControlPath, { force: true });
  if (riskProbe) {
    process.exitCode = 0;
    return;
  }
  const summary = await writeSummary(results, true);
  console.log(JSON.stringify({ type: "summary", summary, summaryPath }));
}

main().catch((error) => {
  if (riskControlPattern.test(error.message) || captchaUnavailablePattern.test(error.message)) process.exitCode = riskExitCode;
  else process.exitCode = 1;
  console.error(JSON.stringify({ ok: false, error: error.message }));
});
