import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";

const workspace = process.cwd();
const protocolClient = path.join(workspace, "scripts", "minimax-protocol-client.mjs");
const imapClient = path.join(workspace, "scripts", "imap.mjs");
const runConfigPath = process.env.RUN_CONFIG_PATH || path.join(workspace, "artifacts", "run-config.json");
const statePath = process.env.STATE_PATH || path.join(workspace, "artifacts", "minimax-session.json");
const flowPath = process.env.FLOW_PATH || path.join(workspace, "artifacts", "minimax-flow.json");
const userCode = process.env.USER_CODE;
const email = process.env.EMAIL;
const password = process.env.PASSWORD;
let captchaTicket = process.env.CAPTCHA_TICKET;
const imapUser = process.env.IMAP_USER;
const imapPassword = process.env.IMAP_PASSWORD;
const pollIntervalMs = Number(process.env.POLL_INTERVAL_MS || 4000);
const pollTimeoutMs = Number(process.env.POLL_TIMEOUT_MS || 180000);
const runConfig = JSON.parse((await fs.readFile(runConfigPath, "utf8").catch(() => "{}")).replace(/^\uFEFF/, ""));
const effectivePassword = password || runConfig.password;
captchaTicket = captchaTicket || runConfig.captchaTicket || JSON.parse(await fs.readFile(statePath, "utf8").catch(() => "{}")).captcha?.ticket || "";

for (const [name, value] of Object.entries({ USER_CODE: userCode, EMAIL: email, PASSWORD: effectivePassword, IMAP_USER: imapUser, IMAP_PASSWORD: imapPassword })) {
  if (!value) throw new Error(`${name} is required`);
}

const baseEnv = {
  ...process.env,
  STATE_PATH: statePath,
  USER_CODE: userCode,
  EMAIL: email,
  PASSWORD: effectivePassword,
  MAIL_TO: email,
};

function run(script, env, timeoutMs = 30000, command = "") {
  return new Promise((resolve, reject) => {
    const args = command ? [script, command] : [script];
    const child = spawn(process.execPath, args, { cwd: workspace, env, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    const timer = setTimeout(() => child.kill("SIGTERM"), timeoutMs);
    child.stdout.setEncoding("utf8").on("data", (chunk) => { stdout += chunk; });
    child.stderr.setEncoding("utf8").on("data", (chunk) => { stderr += chunk; });
    child.on("error", (error) => { clearTimeout(timer); reject(error); });
    child.on("close", (code) => {
      clearTimeout(timer);
      if (code !== 0) return reject(new Error(`${path.basename(script)} ${command || ""} exited ${code}: ${stderr || stdout}`));
      try {
        const json = JSON.parse(stdout);
        resolve({ code, json, stderr });
      } catch (error) {
        reject(new Error(`Invalid JSON from ${script}: ${stderr || stdout || error.message}`));
      }
    });
  });
}

const stepMessages = {
  page: "正在加载设备码授权页",
  status: "正在检查设备码状态",
  "register-device": "正在注册设备",
  "check-account": "正在检查邮箱是否可注册",
  "send-code": "正在发送邮箱验证码",
  "poll-mail": "正在收取邮箱验证码",
  "status-after-login": "正在复核登录状态",
  authorize: "正在提交 OAuth 授权",
  "status-final": "正在确认授权结果",
};

function emitFlowProgress(step) {
  if (step === "create-" + "password") {
    console.error(JSON.stringify({ type: "flow-progress", step, message: "正在创建账号", at: new Date().toISOString() }));
    return;
  }
  console.error(JSON.stringify({ type: "flow-progress", step, message: stepMessages[step] || `正在执行 ${step}`, at: new Date().toISOString() }));
}

function apiError(step, response) {
  const payload = response?.json?.response?.json;
  const info = payload?.statusInfo || payload?.base_resp;
  if (info?.code) return new Error(`${step}: ${info.code} ${info.message}`);
  if (info?.status_code) return new Error(`${step}: ${info.status_code} ${info.status_msg || info.message || ""}`.trim());
  if (response?.json?.response?.status >= 400) return new Error(`${step}: HTTP ${response.json.response.status} ${response.json.response.body || ""}`.trim());
  return null;
}

const steps = [];
function record(name, response) {
  steps.push({ name, at: new Date().toISOString(), response });
  return response;
}

async function saveFlow(status, error) {
  await fs.mkdir(path.dirname(flowPath), { recursive: true });
  await fs.writeFile(flowPath, JSON.stringify({ status, userCode, email, error: error?.message || null, steps }, null, 2), "utf8");
}

function runWithProgress(script, env, timeoutMs, command) {
  const step = command || (path.basename(script) === "imap.mjs" ? "poll-mail" : "protocol");
  emitFlowProgress(step);
  return run(script, env, timeoutMs, command);
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

try {
  record("page", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "page"));
  const status = record("status", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "status")).json.response.json;
  if (status?.error === "login_required") {
    record("register-device", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "register-device"));
    const check = record("check-account", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "check-account"));
    const statusInfo = check.json.response.json?.statusInfo;
    if (statusInfo?.code === 1200056) {
      record("send-code", await runWithProgress(protocolClient, { ...baseEnv, CAPTCHA_TICKET: captchaTicket, CAPTCHA_ACTION: "send_code" }, 30000, "send-code"));
      const sendError = apiError("send-code", steps.at(-1).response);
      if (sendError) throw sendError;
      const deadline = Date.now() + pollTimeoutMs;
      let code = null;
      while (Date.now() < deadline) {
        const mail = await runWithProgress(imapClient, { ...baseEnv, MAIL_SINCE_SECONDS: "600" }, 30000);
        record("poll-mail", mail);
        code = mail.json.code;
        if (code) break;
        await delay(pollIntervalMs);
      }
      if (!code) throw new Error("No MiniMax verification code arrived before timeout");
      record("create-password", await runWithProgress(protocolClient, { ...baseEnv, EMAIL_CODE: code }, 30000, "create-password"));
      const createError = apiError("create-password", steps.at(-1).response);
      if (createError) throw createError;
    } else if (statusInfo?.code) {
      throw new Error(`check-account returned ${statusInfo.code}: ${statusInfo.message}`);
    }
  } else if (!status || status.error) {
    throw new Error(`Unexpected authorization status: ${JSON.stringify(status)}`);
  }

  const finalStatus = record("status-after-login", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "status")).json.response.json;
  if (finalStatus?.status !== "pending") throw new Error(`Authorization is not pending: ${JSON.stringify(finalStatus)}`);
  record("authorize", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "authorize"));
  const authorizeError = apiError("authorize", steps.at(-1).response);
  if (authorizeError) throw authorizeError;
  const approvedStatus = record("status-final", await runWithProgress(protocolClient, { ...baseEnv }, 30000, "status")).json.response.json;
  if (approvedStatus?.status !== "approved") throw new Error(`Authorization was not approved: ${JSON.stringify(approvedStatus)}`);
  await saveFlow("approved");
  console.log(JSON.stringify({ ok: true, userCode, email, status: approvedStatus.status, flowPath }, null, 2));
} catch (error) {
  await saveFlow("failed", error);
  console.error(JSON.stringify({ ok: false, error: error.message, flowPath }, null, 2));
  process.exitCode = 1;
}
