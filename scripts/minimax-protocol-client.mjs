import crypto from "node:crypto";
import fs from "node:fs/promises";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import path from "node:path";
import tls from "node:tls";

const host = "account.minimax.io";
const origin = `https://${host}`;
const command = process.argv[2] || "status";
const statePath = process.env.STATE_PATH || path.join(process.cwd(), "artifacts", "minimax-session.json");
const userCode = process.env.USER_CODE || "";
const email = process.env.EMAIL || "";
const emailCode = process.env.EMAIL_CODE || "";
let captchaTicket = process.env.CAPTCHA_TICKET || "";
let captchaRandStr = process.env.CAPTCHA_RAND_STR || "";
let captchaType = process.env.CAPTCHA_TYPE || "recaptcha";
const captchaAction = process.env.CAPTCHA_ACTION || "send_code";
const password = process.env.PASSWORD || "";
const loginRedirectOverride = process.env.LOGIN_REDIRECT || "";
const proxyUrl = process.env.MINIMAX_PROXY_URL || "";
const requiredEnvironment = (name) => {
  const value = process.env[name] || "";
  if (!value || /^<.+>$/.test(value)) throw new Error(`${name} must be configured locally`);
  return value;
};
const state = await loadState();
captchaTicket = captchaTicket || state.captcha?.ticket || "";
captchaRandStr = captchaRandStr || state.captcha?.randStr || "";
captchaType = process.env.CAPTCHA_TYPE || state.captcha?.captchaType || captchaType;

function withProxy(options) {
  if (!proxyUrl) return options;
  const url = new URL(proxyUrl);
  const scheme = url.protocol.replace(":", "").toLowerCase();
  if (scheme !== "http" && scheme !== "https" && scheme !== "socks5") {
    throw new Error(`Unsupported MINIMAX_PROXY_URL scheme: ${scheme}`);
  }
  return { ...options, proxy: { scheme, host: url.hostname, port: Number(url.port), user: url.username, pass: url.password } };
}

function openConnection(options, secure) {
  if (!options.proxy) {
    return secure ? tls.connect({ host: options.host, port: options.port || 443, servername: options.host }) : net.connect(options.port || 80, options.host);
  }
  const proxy = options.proxy;
  if (proxy.scheme === "socks5") {
    return new Promise((resolve, reject) => {
      const socket = net.connect(proxy.port, proxy.host);
      const methods = proxy.user ? [0, 2] : [0];
      const hostBuffer = Buffer.from(options.host, "utf8");
      const portBuffer = Buffer.alloc(2);
      portBuffer.writeUInt16BE(options.port || 443);
      const queue = [Buffer.from([5, methods.length, ...methods]), Buffer.from([5, 1, 0, 3, hostBuffer.length, ...hostBuffer, ...portBuffer])];
      let stage = 0;
      let buffer = Buffer.alloc(0);
      const fail = (message) => { socket.destroy(); reject(new Error(message)); };
      const send = () => {
        const limit = stage === 0 ? 1 : queue.length;
        for (let index = 0; index < limit && queue.length; index += 1) socket.write(queue.shift());
      };
      socket.on("connect", send);
      socket.setTimeout(15000, () => fail("SOCKS5 connection timed out"));
      socket.on("data", (chunk) => {
        buffer = Buffer.concat([buffer, chunk]);
        if (buffer.length >= 2 && buffer[0] === 5 && buffer[1] === 2) {
          const user = Buffer.from(proxy.user || "", "utf8");
          const pass = Buffer.from(proxy.pass || "", "utf8");
          queue.splice(1, 0, Buffer.from([1, user.length, ...user, pass.length, ...pass]));
          buffer = buffer.subarray(2);
          send();
          return;
        }
        if (buffer.length >= 2 && buffer[0] === 1 && buffer[1] === 0) {
          stage = 1;
          buffer = buffer.subarray(2);
          send();
          return;
        }
        if (stage === 0 && buffer.length >= 2) {
          const method = buffer[1];
          buffer = buffer.subarray(2);
          if (method !== 0) return fail("SOCKS5 authentication method rejected");
          stage = 1;
          send();
        }
        if (stage === 1 && buffer.length >= 4) {
          if (buffer[1] !== 0) return fail("SOCKS5 CONNECT failed");
          const length = [0, 4, 1, 0, 0, 0, 0, 1][buffer[3]] || 0;
          if (buffer.length < 4 + length + 2) return;
          const remainder = buffer.subarray(4 + length + 2);
          socket.removeAllListeners("data");
          socket.removeAllListeners("connect");
          socket.setTimeout(0);
          const connected = secure ? tls.connect({ socket, servername: options.host }) : socket;
          if (remainder.length) connected.unshift?.(remainder);
          if (!secure) resolve(connected);
          else connected.once("secureConnect", () => resolve(connected));
          socket.removeAllListeners("error");
          connected.on("error", reject);
        }
      });
      socket.once("error", reject);
    });
  }
  return new Promise((resolve, reject) => {
    const headers = { Host: `${options.host}:${options.port || 443}` };
    if (proxy.user) headers["Proxy-Authorization"] = `Basic ${Buffer.from(`${decodeURIComponent(proxy.user)}:${decodeURIComponent(proxy.pass || "")}`).toString("base64")}`;
    const connectOptions = {
      host: proxy.host,
      port: proxy.port,
      method: "CONNECT",
      path: `${options.host}:${options.port || 443}`,
      headers,
    };
    const request = proxy.scheme === "https"
      ? https.request({ ...connectOptions, rejectUnauthorized: false })
      : http.request(connectOptions);
    request.once("connect", (response, socket) => {
      if (response.statusCode !== 200) return reject(new Error(`Proxy CONNECT failed: HTTP ${response.statusCode}`));
      const connected = tls.connect({ socket, servername: options.host });
      connected.once("secureConnect", () => resolve(connected));
      connected.once("error", reject);
    });
    request.once("error", reject);
    request.end();
  });
}

const constants = {
  appId: "3001",
  bizId: "3",
  versionCode: "22201",
  client: "web",
  loginTypeEmailCode: "21",
  loginTypePassword: "20",
  signatureSalt: requiredEnvironment("MINIMAX_SIGNATURE_SALT"),
  uniqueUserIdKey: "mmx_account_UNIQUE_USER_ID",
  deviceIdKey: "mmx_account_USER_HARD_WARE_INFO",
  userDetailKey: "user_detail_agent",
};

const browserProfile = {
  userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.8103.51 Safari/537.36",
  osName: "Windows",
  browserName: "Chrome",
  browserLanguage: "en-US",
  browserPlatform: "Win32",
  deviceMemory: 8,
  cpuCoreNum: 8,
  screenWidth: 3440,
  screenHeight: 1440,
};

function md5(value) {
  return crypto.createHash("md5").update(value).digest("hex");
}

function encodePassword(value) {
  const key = crypto.createPublicKey(`-----BEGIN PUBLIC KEY-----\nMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDF5ndG2/UB4L5tbvQaNLHSoBTW\nDKbrNBuOmUIP23eCmC2ELMx3kppEikxTp5cV8NxUZl6ii+KLwKugioAXApzypHXb\ngXbq13kTKA7OCA1xtAoMdH9cltjBiFAUJlgmVjr0MuJCknhVAjWLjCVRHege+Atl\ngkUBUeGa9O+cWcPEwQIDAQAB\n-----END PUBLIC KEY-----`);
  return crypto.publicEncrypt({ key, padding: crypto.constants.RSA_PKCS1_PADDING }, Buffer.from(value, "utf8")).toString("base64");
}

function parseCookie(setCookie) {
  const pair = String(setCookie || "").split(";")[0];
  const separator = pair.indexOf("=");
  if (separator < 1) return null;
  return { name: pair.slice(0, separator).trim(), value: pair.slice(separator + 1).trim() };
}

function loadState() {
  return fs.readFile(statePath, "utf8").then(JSON.parse).catch(() => ({
    cookies: {},
    localStorage: {},
  }));
}

async function saveState(state) {
  await fs.mkdir(path.dirname(statePath), { recursive: true });
  await fs.writeFile(statePath, JSON.stringify(state, null, 2), "utf8");
}

function cookieHeader(state) {
  return Object.entries(state.cookies).map(([name, value]) => `${name}=${value}`).join("; ");
}

function addResponseCookies(state, response) {
  for (const setCookie of response.headers["set-cookie"] || []) {
    const cookie = parseCookie(setCookie);
    if (cookie) state.cookies[cookie.name] = cookie.value;
  }
}

function baseHeaders(state, extra = {}) {
  return {
    "User-Agent": browserProfile.userAgent,
    Accept: "application/json, text/plain, */*",
    "Accept-Language": "en-US,en;q=0.9",
    Origin: origin,
    Referer: `${origin}/unified-login`,
    ...(cookieHeader(state) ? { Cookie: cookieHeader(state) } : {}),
    ...extra,
  };
}

function signedRequest(state, { path: requestPath, method = "POST", body = {}, params = {}, signed = true }) {
  return new Promise((resolve, reject) => {
    const unix = Date.now();
    const timestamp = Math.floor(unix / 1000);
    const payload = method.toUpperCase() === "POST" ? JSON.stringify(body) : "";
    const query = new URLSearchParams();
    if (signed) {
      const signedParams = {
        device_platform: "web",
        biz_id: constants.bizId,
        app_id: constants.appId,
        version_code: constants.versionCode,
        unix: String(unix),
        timezone_offset: String(-new Date().getTimezoneOffset()),
        sys_language: "en",
        lang: "en",
        uuid: state.localStorage[constants.uniqueUserIdKey] || "",
        device_id: state.localStorage[constants.deviceIdKey] || "0",
        os_name: browserProfile.osName,
        browser_name: browserProfile.browserName,
        device_memory: String(browserProfile.deviceMemory),
        cpu_core_num: String(browserProfile.cpuCoreNum),
        browser_language: browserProfile.browserLanguage,
        browser_platform: browserProfile.browserPlatform,
        screen_width: String(browserProfile.screenWidth),
        screen_height: String(browserProfile.screenHeight),
        user_id: state.localStorage[constants.userDetailKey]?.realUserID || "0",
        client: constants.client,
      };
      for (const [key, value] of Object.entries(signedParams)) query.set(key, value);
    }
    for (const [key, value] of Object.entries(params)) query.set(key, value);
    const fullPath = `${requestPath}${query.size ? `?${query}` : ""}`;
    const yyInput = `${encodeURIComponent(fullPath)}_${method.toUpperCase() === "POST" ? payload : "{}"}${md5(String(unix))}${requiredEnvironment("MINIMAX_YY_SUFFIX")}`;
    const headers = baseHeaders(state, {
      "Content-Type": "application/json",
      ...(payload ? { "Content-Length": String(Buffer.byteLength(payload)) } : {}),
      ...(signed ? {
        "x-timestamp": String(timestamp),
        "x-signature": md5(`${timestamp}${constants.signatureSalt}${payload}`),
        yy: md5(yyInput),
      } : {}),
    });
    const options = withProxy({ host, port: 443, path: fullPath, method, headers, createConnection: proxyUrl ? (options, callback) => {
      openConnection(options, true).then((socket) => callback(null, socket)).catch(callback);
    } : undefined });
    const request = https.request(options, (response) => {
      addResponseCookies(state, response);
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        let json = null;
        try { json = JSON.parse(text); } catch {}
        resolve({ status: response.statusCode, headers: response.headers, body: text.slice(0, 8000), json });
      });
    });
    request.setTimeout(20000, () => request.destroy(new Error("request timeout")));
    request.on("error", reject);
    if (payload) request.write(payload);
    request.end();
  });
}

function plainRequest(state, { url, method = "GET", body = null, headers = {} }) {
  return new Promise((resolve, reject) => {
    const target = new URL(url, origin);
    const payload = body === null ? "" : JSON.stringify(body);
    const options = withProxy({
      host: target.hostname,
      port: 443,
      path: `${target.pathname}${target.search}`,
      method,
      headers: baseHeaders(state, {
        Accept: "application/json, text/plain, */*",
        ...(payload ? { "Content-Type": "application/json", "Content-Length": String(Buffer.byteLength(payload)) } : {}),
        ...headers,
      }),
      createConnection: proxyUrl ? (options, callback) => {
        openConnection(options, true).then((socket) => callback(null, socket)).catch(callback);
      } : undefined,
    });
    const request = https.request(options, (response) => {
      addResponseCookies(state, response);
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => {
        const text = Buffer.concat(chunks).toString("utf8");
        let json = null;
        try { json = JSON.parse(text); } catch {}
        resolve({ status: response.statusCode, headers: response.headers, body: text.slice(0, 8000), json });
      });
    });
    request.setTimeout(20000, () => request.destroy(new Error("request timeout")));
    request.on("error", reject);
    if (payload) request.write(payload);
    request.end();
  });
}

function loginRedirect(code) {
  if (loginRedirectOverride) return loginRedirectOverride;
  const query = new URLSearchParams({ user_code: code });
  return `/oauth-authorize?${query}`;
}

function requireUserCode() {
  if (!userCode) throw new Error("USER_CODE is required");
}

function requireEmail() {
  if (!email) throw new Error("EMAIL is required");
}

function requireCaptcha() {
  if (!captchaTicket) throw new Error("CAPTCHA_TICKET from the browser's reCAPTCHA Enterprise token is required");
}

function requirePassword() {
  if (!password) throw new Error("PASSWORD is required for the new-user create-password flow");
}

function loginBody() {
  const body = emailCode && !password
    ? { loginType: constants.loginTypeEmailCode, email, code: emailCode, deviceID: state.localStorage[constants.deviceIdKey] }
    : { loginType: constants.loginTypePassword, email, authToken: encodePassword(password), code: emailCode, deviceID: state.localStorage[constants.deviceIdKey] };
  body.login_redirect = loginRedirect(userCode);
  return body;
}

let response;

switch (command) {
  case "page": {
    requireUserCode();
    const url = `${origin}/oauth-authorize?${new URLSearchParams({ client_id: "mcode-public", user_code: userCode })}`;
    response = await plainRequest(state, { url, headers: { Accept: "text/html,application/xhtml+xml" } });
    break;
  }
  case "status": {
    requireUserCode();
    const query = new URLSearchParams({ user_code: userCode, app_id: constants.appId });
    response = await plainRequest(state, { url: `/oauth2/device/authorize?${query}`, headers: { "Cache-Control": "no-store" } });
    break;
  }
  case "register-device": {
    const uuid = state.localStorage[constants.uniqueUserIdKey] || crypto.randomUUID();
    state.localStorage[constants.uniqueUserIdKey] = uuid;
    response = await signedRequest(state, { path: "/v1/api/user/device/register", body: { uuid } });
    const deviceId = response.json?.data?.deviceIDStr || response.json?.data?.deviceIdStr;
    if (deviceId) state.localStorage[constants.deviceIdKey] = deviceId;
    break;
  }
  case "check-account": {
    requireEmail();
    requireUserCode();
    response = await signedRequest(state, {
      path: "/oauth2/login",
      body: {
        loginType: constants.loginTypePassword,
        email,
        authToken: null,
        deviceID: state.localStorage[constants.deviceIdKey],
        login_redirect: loginRedirect(userCode),
      },
    });
    break;
  }
  case "send-code": {
    requireEmail();
    requireCaptcha();
    if (state.captcha && state.captcha.action && state.captcha.action !== captchaAction) {
      throw new Error(`Bridged CAPTCHA action is ${state.captcha.action}, but ${captchaAction} was requested`);
    }
    response = await signedRequest(state, {
      path: "/v1/api/user/login/sms/send",
      body: { email, phone: "", randStr: captchaRandStr, ticket: captchaTicket, captchaType },
    });
    break;
  }
  case "login": {
    requireEmail();
    requireUserCode();
    if (!emailCode && !password) throw new Error("EMAIL_CODE or PASSWORD is required");
    response = await signedRequest(state, { path: "/oauth2/login", body: loginBody() });
    const loginData = response.json?.data;
    if (loginData) {
      state.localStorage[constants.userDetailKey] = loginData;
      state.lastLogin = { userCode, email, at: new Date().toISOString() };
    }
    break;
  }
  case "authorize": {
    requireUserCode();
    const status = await plainRequest(state, { url: `/oauth2/device/authorize?${new URLSearchParams({ user_code: userCode, app_id: constants.appId })}`, headers: { "Cache-Control": "no-store" } });
    const pending = status.json?.status === "pending" ? status.json : null;
    if (!pending) {
      response = { status: status.status, body: status.body, json: status.json, note: "authorization is not pending" };
      break;
    }
    const groups = Array.isArray(pending.groups) ? pending.groups : [];
    const defaultGroups = groups.filter((group) => group.is_default);
    if (defaultGroups.length !== 1 || !defaultGroups[0].can_authorize) {
      throw new Error("No single default group is available to approve authorization");
    }
    response = await plainRequest(state, {
      url: "/oauth2/device/authorize",
      method: "POST",
      body: { user_code: userCode, decision: "approve", group_id: defaultGroups[0].group_id },
      headers: { "Cache-Control": "no-store" },
    });
    break;
  }
  case "deny": {
    requireUserCode();
    response = await plainRequest(state, {
      url: "/oauth2/device/authorize",
      method: "POST",
      body: { user_code: userCode, decision: "deny" },
      headers: { "Cache-Control": "no-store" },
    });
    break;
  }
  case "create-password": {
    requireEmail();
    requireUserCode();
    requirePassword();
    response = await signedRequest(state, { path: "/oauth2/login", body: loginBody() });
    const loginData = response.json?.data;
    if (loginData) {
      state.localStorage[constants.userDetailKey] = loginData;
      state.lastLogin = { userCode, email, at: new Date().toISOString() };
    }
    break;
  }
  default:
    throw new Error(`Unknown command: ${command}`);
}

await saveState(state);
console.log(JSON.stringify({
  command,
  statePath,
  userCode: userCode || null,
  cookies: Object.keys(state.cookies),
  response: response,
}, null, 2));
