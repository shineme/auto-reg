import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import net from "node:net";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const panelBase = process.env.PANEL_BASE || "https://panel.example.com";
const panelKey = process.env.PANEL_KEY || "";
const imapUser = process.env.IMAP_USER || "";
const imapPassword = process.env.IMAP_PASSWORD || "";
const mailDomain = process.env.MAIL_DOMAIN || "example.com";
const mailPrefix = process.env.MAIL_PREFIX || "mm";
const proxyUrl = process.env.MINIMAX_PROXY_URL || "";
const cdpHost = process.env.CDP_HOST || "127.0.0.1";
const chromePath = process.env.CHROME_PATH || "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const batchRoot = process.env.BATCH_ROOT || path.join(workspace, "artifacts", "fast-batch");
const batchId = process.env.BATCH_ID || new Date().toISOString().replace(/[-:T]/g, "").slice(0, 14);
const root = path.join(batchRoot, batchId);
const chromeProfileRoot = process.env.CHROME_PROFILE_ROOT || path.join(root, "chrome-profiles");
const summaryPath = path.join(root, "summary.json");
const count = Number(process.env.COUNT || 1);
const targetSuccessCount = Math.max(1, Number(process.env.TARGET_SUCCESS_COUNT || count));
const cdpStartupTimeoutMs = Math.max(5000, Number(process.env.CDP_STARTUP_TIMEOUT_MS || 30000));
const restartCount = Number(process.env.MINIMAX_RESTART_COUNT || 0);
const dryRun = process.env.DRY_RUN === "1";
const panelCheckOnly = process.env.PANEL_CHECK_ONLY === "1";
const signInPollSeconds = Number(process.env.SIGN_IN_POLL_SECONDS || 90);
const captchaReadyAttempts = Number(process.env.CAPTCHA_READY_ATTEMPTS || 60);
const captchaPrewarmUrl = process.env.CAPTCHA_PREWARM_URL || "https://account.minimax.io/unified-login";
const captchaUnavailablePattern = /reCAPTCHA Enterprise did not become ready|reCAPTCHA Enterprise returned no ticket|CDP browser did not become ready|CDP version HTTP/i;
const siteKey = process.env.MINIMAX_RECAPTCHA_SITE_KEY || "";
const captchaAction = "send_code";
const captchaPrewarmHost = "account.minimax.io";
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

function withTimeout(promise, timeoutMs, label) {
  let timer;
  return Promise.race([
    Promise.resolve(promise).finally(() => clearTimeout(timer)),
    new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`${label} timed out`)), timeoutMs);
    }),
  ]);
}

async function pathExists(filePath) {
  try {
    await fs.stat(filePath);
    return true;
  } catch {
    return false;
  }
}

function processIsAlive(processId) {
  if (!processId) return false;
  try {
    process.kill(processId, 0);
    return true;
  } catch (error) {
    return error.code === "EPERM";
  }
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
  const value = {
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
  };
  if (result.retryable !== undefined) value.retryable = Boolean(result.retryable);
  return value;
}

function updateProgressState(results, current) {
  progressState.succeeded = results.filter((result) => result.ok).length;
  progressState.failed = results.filter((result) => !result.ok).length;
  progressState.attempts = results.length;
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
      if (!result.ok && captchaUnavailablePattern.test(result.error || "")) {
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

async function cdpVersion(port) {
  const response = await fetch(`http://${cdpHost}:${port}/json/version`);
  if (!response.ok) throw new Error(`CDP version HTTP ${response.status}`);
  return response.json();
}

async function waitForCdpPort(profileDir) {
  const portPath = path.join(profileDir, "DevToolsActivePort");
  const deadline = Date.now() + cdpStartupTimeoutMs;
  while (Date.now() < deadline) {
    try {
      const [portText] = (await fs.readFile(portPath, "utf8")).split(/\r?\n/);
      const port = Number(portText);
      if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error("invalid DevToolsActivePort");
      return await cdpVersion(port);
    } catch {
      await delay(200);
    }
  }
  throw new Error(`CDP browser did not become ready (${path.basename(profileDir)})`);
}

async function reserveCdpPort() {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, cdpHost, resolve);
  });
  const port = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return port;
}

function chromeProxyArgument() {
  if (!proxyUrl) return "";
  const parsed = new URL(proxyUrl);
  if (!["http:", "https:", "socks5:"].includes(parsed.protocol)) throw new Error(`Unsupported Chrome proxy scheme: ${parsed.protocol}`);
  if (parsed.protocol === "socks5:") return `socks5://${parsed.host}`;
  const credentials = parsed.username
    ? `${decodeURIComponent(parsed.username)}${parsed.password ? `:${decodeURIComponent(parsed.password)}` : ""}@`
    : "";
  return `${parsed.protocol}//${credentials}${parsed.host}`;
}

async function startCdpBrowser(profileDir) {
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
  if (proxyArgument) args.push(`--proxy-server=${proxyArgument}`);
  args.push("about:blank");
  const child = spawn(chromePath, args, { stdio: "ignore", windowsHide: true });
  const browser = { child, profileDir, version: null, port: 0 };
  try {
    browser.version = await waitForCdpPort(profileDir);
    browser.port = Number(new URL(browser.version.webSocketDebuggerUrl).port);
    return browser;
  } catch (error) {
    await terminateProcessTree(child);
    await removeCdpProfile(browser);
    throw error;
  }
}

function cdpCall(socket, method, params = {}, sessionId) {
  const id = nextCommandId++;
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pendingCommands.delete(id);
      reject(new Error(`CDP ${method} timed out`));
    }, 30000);
    pendingCommands.set(id, {
      resolve: (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      reject: (error) => {
        clearTimeout(timer);
        reject(error);
      },
    });
    const message = { id, method, params };
    if (sessionId) message.sessionId = sessionId;
    try {
      socket.send(JSON.stringify(message));
    } catch (error) {
      clearTimeout(timer);
      pendingCommands.delete(id);
      reject(error);
    }
  });
}

function connectCdpSocket(version) {
  const socket = new WebSocket(version.webSocketDebuggerUrl);
  let readySettled = false;
  let rejectReady;
  const ready = new Promise((resolve, reject) => {
    rejectReady = reject;
    socket.onopen = () => {
      readySettled = true;
      resolve();
    };
    socket.onerror = () => {
      if (readySettled) return;
      readySettled = true;
      reject(new Error("CDP WebSocket connection failed"));
    };
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
    if (!readySettled) {
      readySettled = true;
      rejectReady(new Error("CDP socket closed before open"));
    }
    for (const { reject } of pendingCommands.values()) reject(new Error("CDP socket closed"));
    pendingCommands.clear();
  };
  return { socket, ready, call: (method, params, sessionId) => cdpCall(socket, method, params, sessionId) };
}

async function connectCdpBrowser(version) {
  const cdp = connectCdpSocket(version);
  await cdp.ready;
  return cdp;
}

async function terminateProcessTree(child) {
  if (!child?.pid || child.exitCode !== null || child.signalCode !== null) return;
  if (process.platform === "win32") {
    await new Promise((resolve) => {
      const killer = spawn("taskkill.exe", ["/PID", String(child.pid), "/T", "/F"], { stdio: "ignore", windowsHide: true });
      killer.once("close", resolve);
      killer.once("error", resolve);
    });
  } else {
    try {
      child.kill("SIGTERM");
    } catch {}
  }
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline && processIsAlive(child.pid)) await delay(100);
}

async function stopCdpBrowser(browser, cdp) {
  if (cdp) {
    try {
      await withTimeout(cdp.call("Browser.close"), 5000, "CDP Browser.close");
    } catch {}
  }
  await terminateProcessTree(browser?.child);
  try {
    cdp?.socket?.close();
  } catch {}
}

async function removeCdpProfile(browser) {
  if (!browser?.profileDir) return;
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    try {
      await fs.rm(browser.profileDir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
      if (!(await pathExists(browser.profileDir))) return;
    } catch {
      await delay(250);
    }
  }
  await fs.mkdir(root, { recursive: true });
  await fs.appendFile(
    path.join(root, "chrome-profile-cleanup-failures.jsonl"),
    `${JSON.stringify({ at: new Date().toISOString(), profileDir: browser.profileDir, chromePid: browser.child?.pid || 0 })}\n`,
    "utf8",
  );
  emitProgress("chrome-cleanup", "临时 Chrome profile 清理失败，请检查输出目录", { current: 0 });
}

async function withCdpBrowser(owner, worker) {
  const label = String(owner?.index ?? owner?.name ?? "session").replace(/[^a-zA-Z0-9_-]/g, "-");
  const profileDir = path.join(chromeProfileRoot, `${batchId}-${label}-${process.pid}-${crypto.randomBytes(4).toString("hex")}`);
  let browser;
  let cdp;
  try {
    browser = await startCdpBrowser(profileDir);
    cdp = await connectCdpBrowser(browser.version);
    return await worker(cdp, browser);
  } finally {
    try {
      await stopCdpBrowser(browser, cdp);
    } finally {
      await removeCdpProfile(browser);
    }
  }
}

async function closeBrowserContext(cdp, context) {
  await disposeBrowserContext(cdp, context?.browserContextId);
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

async function createAccountContext(cdp, task) {
  let browserContextId;
  const observed = {
    profileDir: task.browserProfileDir || "",
    cdpPort: Number(task.browserCdpPort || 0),
    chromePid: Number(task.browserChromePid || 0),
  };
  try {
    ({ browserContextId } = await cdp.call("Target.createBrowserContext", { disposeOnDetach: true }));
    const { targetId } = await cdp.call("Target.createTarget", { url: "about:blank", browserContextId });
    const { sessionId } = await cdp.call("Target.attachToTarget", { targetId, flatten: true });
    await cdp.call("Page.enable", {}, sessionId);
    await cdp.call("Runtime.enable", {}, sessionId);
    await cdp.call("Network.enable", {}, sessionId);
    await cdp.call("Network.setCacheDisabled", { cacheDisabled: true }, sessionId);
    await cdp.call("Emulation.setDeviceMetricsOverride", {
      width: screenWidth,
      height: screenHeight,
      deviceScaleFactor: 1,
      mobile: false,
    }, sessionId);
    await cdp.call("Page.navigate", { url: task.prewarmUrl || prewarmUrl(task.userCode, task.loginRedirect) }, sessionId);
    let captchaPageReady = false;
    for (let attempt = 0; attempt < captchaReadyAttempts; attempt += 1) {
      const pageStateText = await evaluate(cdp, sessionId, `(function () {
        try {
          return JSON.stringify({
            href: location.href,
            ready: document.readyState,
            grecaptcha: typeof window.grecaptcha !== "undefined" && typeof window.grecaptcha.enterprise !== "undefined",
            host: location.host
          });
        } catch {
          return "{}";
        }
      })()`);
      const pageState = JSON.parse(pageStateText || "{}");
      if (pageState.grecaptcha && pageState.ready === "complete" && pageState.host === captchaPrewarmHost) {
        captchaPageReady = true;
        break;
      }
      await delay(500);
    }
    if (!captchaPageReady) throw new Error("reCAPTCHA Enterprise did not become ready");
    const ticket = await evaluate(cdp, sessionId, `(async () => {
      await new Promise((resolve) => window.grecaptcha.enterprise.ready(resolve));
      return await window.grecaptcha.enterprise.execute("${siteKey}", { action: "${captchaAction}" });
    })()`, { awaitPromise: true, userGesture: true });
    if (typeof ticket !== "string" || !ticket) throw new Error("reCAPTCHA Enterprise returned no ticket");
    const cookieResult = await cdp.call("Network.getCookies", { urls: [`https://${captchaPrewarmHost}/`] }, sessionId);
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
    const stateWithBrowser = {
      ...state,
      browser: { ...observed, browserContextId, targetId, sessionId, prewarmUrl: task.prewarmUrl || prewarmUrl(task.userCode, task.loginRedirect) },
    };
    await writeJson(task.statePath, stateWithBrowser);
    return { browserContextId, sessionId, observed };
  } catch (error) {
    await disposeBrowserContext(cdp, browserContextId);
    throw error;
  }
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

async function processAccount(task, beforeUsers, progressHandler) {
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
    const result = {
      ok: false,
      email: task.email,
      password: task.password,
      userCode: task.userCode,
      signInId: task.signInId,
      error: error.message,
      runDir: task.runDir,
      startedAt,
      completedAt: new Date().toISOString(),
    };
    await writeJson(task.resultPath, result);
    return { index: task.index, ok: false, error: error.message, runDir: task.runDir, startedAt };
  }
}

async function writeSummary(results, finished = false) {
  const succeeded = results.filter((result) => result.ok).length;
  const failed = results.filter((result) => !result.ok).length;
  const summary = {
    batchId,
    requested: targetSuccessCount,
    completed: succeeded,
    done: succeeded,
    succeeded,
    failed,
    attempts: results.length,
    updatedAt: new Date().toISOString(),
    results,
  };
  if (finished || succeeded >= targetSuccessCount) summary.finishedAt = summary.updatedAt;
  await writeJson(summaryPath, summary);
  return summary;
}

async function runDryRun() {
  await fs.mkdir(root, { recursive: true });
  const dummyCode = process.env.DRY_RUN_USER_CODE || "TEST-CODE";
  const task = {
    name: "dry-run",
    prewarmUrl: prewarmUrl(dummyCode),
    statePath: path.join(root, "dryrun-session.json"),
  };
  const result = await withCdpBrowser(task, async (cdp, browser) => {
    const context = await createAccountContext(cdp, {
      ...task,
      browserProfileDir: browser.profileDir,
      browserCdpPort: browser.port,
      browserChromePid: browser.child.pid,
    });
    const state = await readJson(task.statePath);
    const candidate = {
      ok: true,
      ticketLength: state.captcha?.ticket?.length || 0,
      cookieNames: Object.keys(state.cookies || {}),
      uuid: state.localStorage?.[uniqueUserIdKey] || null,
      deviceId: state.localStorage?.[deviceIdKey] || null,
      statePath: task.statePath,
      browser: state.browser,
    };
    await closeBrowserContext(cdp, context);
    return candidate;
  });
  await writeJson(path.join(root, "dryrun.json"), result);
  console.log(JSON.stringify(result));
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
  await fs.mkdir(chromeProfileRoot, { recursive: true });
  await panelFetch(`/?k=${encodeURIComponent(panelKey)}&view=providers&edit=minimax-code-global`, {}, false);
  const existing = await loadExistingResults();
  const results = existing.results;
  let nextIndex = existing.nextIndex;
  updateProgressState(results, nextIndex);
  await writeSummary(results);
  emitProgress("resume", `已恢复 ${progressState.succeeded}/${targetSuccessCount} 个成功账号，正在继续补齐`);
  while (progressState.succeeded < targetSuccessCount) {
      progressState.current = nextIndex;
      emitProgress("creating-code", `正在创建第 ${nextIndex} 个账号设备码`, { current: nextIndex });
      const task = await createTask(nextIndex);
      nextIndex += 1;
      await initializeTask(task);
      const beforeUsers = (await providerUsers()).users;
      emitProgress("captcha", `正在为 ${task.email} 创建独立临时 Chrome profile 并预热 reCAPTCHA 票据`, { current: task.index });
      let result;
      try {
        await withCdpBrowser(task, async (cdp, browser) => {
          task.browserProfileDir = browser.profileDir;
          task.browserCdpPort = browser.port;
          task.browserChromePid = browser.child.pid;
          task.context = await createAccountContext(cdp, task);
          result = await processAccount(task, beforeUsers, (step, message) => emitProgress(step, message, { current: task.index }));
          await closeBrowserContext(cdp, task.context);
        });
      } catch (error) {
        if (!result) {
          task.prewarmError = error.message;
          result = {
            index: task.index,
            ok: false,
            error: task.prewarmError,
            runDir: task.runDir,
          };
          if (!captchaUnavailablePattern.test(task.prewarmError)) {
            await writeJson(task.resultPath, { ok: false, error: task.prewarmError, email: task.email, userCode: task.userCode, signInId: task.signInId, runDir: task.runDir, completedAt: new Date().toISOString() });
          }
        }
      }
      results.push(result);
      updateProgressState(results, nextIndex);
      await writeSummary(results);
      emitAccountResult(result);
      if (captchaUnavailablePattern.test(result.error || "")) {
        emitProgress("captcha-retry", "本轮 reCAPTCHA 票据环境异常，守护进程将自动恢复", { current: task.index });
        process.exitCode = 1;
        return;
      }
      emitProgress("account-done", result.ok ? `账号注册成功：${result.user || result.email}` : `账号未成功：${result.error || "未知错误"}`, { current: task.index });
  }
  const summary = await writeSummary(results, true);
  console.log(JSON.stringify({ type: "summary", summary, summaryPath }));
}

main().catch((error) => {
  process.exitCode = 1;
  console.error(JSON.stringify({ ok: false, error: error.message }));
});
