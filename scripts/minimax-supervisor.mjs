import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const batchRoot = path.resolve(process.env.BATCH_ROOT || path.join(workspace, "artifacts", "gui-batch-200"));
const targetSuccessCount = Number(process.env.TARGET_SUCCESS_COUNT || process.env.COUNT || 200);
const batchId = process.env.BATCH_ID || "resume-200";
const root = path.join(batchRoot, batchId);
const lockPath = path.join(batchRoot, "supervisor.lock");
const statusPath = path.join(batchRoot, "supervisor.status.json");
const progressPath = path.join(batchRoot, "supervisor.progress.json");
const ledgerPath = path.join(batchRoot, "resume-ledger.json");
const summaryPath = path.join(root, "summary.json");
const batchScript = path.resolve(process.env.MINIMAX_BATCH_SCRIPT || path.join(workspace, "scripts", "minimax-fast-batch.mjs"));
const riskControlPath = path.join(root, "risk-control.json");
const riskStatePath = path.join(batchRoot, "risk-state.json");
const riskCooldownSeconds = Math.max(1, Number(process.env.MINIMAX_RISK_COOLDOWN_SECONDS || 1800));
const riskExitCode = 75;
const riskControlPattern = /1200063|人机验证未通过|Verification failed, please try again/i;
const captchaUnavailablePattern = /reCAPTCHA Enterprise did not become ready|reCAPTCHA Enterprise returned no ticket|CDP browser did not become ready|CDP version HTTP|reCAPTCHA 票据环境暂不可用|ECONNRESET|socket connection was closed unexpectedly|The operation was aborted due to timeout|fetch failed/i;
const pid = process.pid;
const startedAt = new Date().toISOString();
let progressSequence = 0;
let progressQueue = Promise.resolve();
let heartbeatFields = {};

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function writeJson(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, JSON.stringify(value, null, 2), "utf8");
}

function publishProgress(event) {
  const payload = { ...event, supervisorPid: pid, supervisorStartedAt: startedAt, eventId: `${pid}-${startedAt}-${++progressSequence}` };
  console.log(JSON.stringify(payload));
  const tempPath = `${progressPath}.tmp`;
  progressQueue = progressQueue
    .then(async () => {
      await writeJson(tempPath, payload);
      await fs.rename(tempPath, progressPath);
    })
    .catch(() => {});
  return progressQueue;
}

async function readJson(filePath, fallback) {
  try {
    return JSON.parse(await fs.readFile(filePath, "utf8"));
  } catch {
    return fallback;
  }
}

async function processExists(candidate) {
  if (!candidate || candidate === pid) return false;
  try {
    process.kill(candidate, 0);
    return true;
  } catch (error) {
    return error.code === "EPERM";
  }
}

async function acquireLock() {
  await fs.mkdir(batchRoot, { recursive: true });
  for (let attempt = 0; attempt < 2; attempt += 1) {
    try {
      const handle = await fs.open(lockPath, "wx");
      await handle.writeFile(JSON.stringify({ pid, startedAt, batchId, targetSuccessCount }, null, 2), "utf8");
      await handle.close();
      return true;
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      const lock = await readJson(lockPath, {});
      if (!(await processExists(lock.pid))) {
        await fs.rm(lockPath, { force: true });
        continue;
      }
      return false;
    }
  }
  return false;
}

async function releaseLock() {
  const lock = await readJson(lockPath, {});
  if (lock.pid === pid) await fs.rm(lockPath, { force: true });
}

function progressEvent(fields = {}) {
  return {
    type: "progress",
    batchId,
    count: targetSuccessCount,
    done: 0,
    ok: 0,
    failed: 0,
    attempts: 0,
    current: 0,
    restartCount: 0,
    riskProbeCount: 0,
    phase: "supervisor",
    message: "守护进程正在工作",
    updatedAt: new Date().toISOString(),
    ...fields,
  };
}

function printStructuredProgress(line, restartCount, riskProbeCount) {
  let event;
  try {
    event = JSON.parse(line);
  } catch {
    return;
  }
  if (!event || typeof event !== "object") return;
  if (event.type === "progress" || event.type === "account-result") {
    publishProgress({ ...event, batchId, restartCount, riskProbeCount });
  }
}

async function collectResults() {
  const entries = await fs.readdir(batchRoot, { withFileTypes: true }).catch(() => []);
  const successes = new Map();
  const failures = [];
  for (const batch of entries) {
    if (!batch.isDirectory() || (!/^\d{8}-\d{6}$/.test(batch.name) && batch.name !== batchId)) continue;
    const batchPath = path.join(batchRoot, batch.name);
    const accounts = await fs.readdir(batchPath, { withFileTypes: true }).catch(() => []);
    for (const account of accounts) {
      if (!account.isDirectory() || !account.name.startsWith("account-")) continue;
      const resultPath = path.join(batchPath, account.name, "result.json");
      let result;
      try {
        result = JSON.parse(await fs.readFile(resultPath, "utf8"));
      } catch {
        continue;
      }
      if (typeof result.ok !== "boolean") continue;
      if (!result.ok && result.riskBlocked) continue;
      if (result.ok) {
        const key = result.email || result.userCode || result.signInId || result.runDir;
        if (!successes.has(key)) successes.set(key, result);
      } else {
        failures.push({ batch: batch.name, account: account.name, error: result.error || "" });
      }
    }
  }
  return { successes: [...successes.values()], failures };
}

async function prepareResumeBatch() {
  await fs.mkdir(root, { recursive: true });
  const { successes, failures } = await collectResults();
  const previousSummary = await readJson(summaryPath, {});
  const previousFailed = Number(previousSummary.failed || 0);
  const previousAttempts = Number(previousSummary.attempts || previousSummary.completed || 0);
  const previousRestartCount = Number(previousSummary.restartCount || 0);
  const ledger = {
    batchId,
    targetSuccessCount,
    succeeded: successes.length,
    failedAttempts: failures.length,
    successes,
    failures,
    updatedAt: new Date().toISOString(),
  };
  await writeJson(ledgerPath, ledger);
  const existing = await fs.readdir(root, { withFileTypes: true }).catch(() => []);
  const kept = new Set();
  for (const entry of existing) {
    if (!entry.isDirectory() || !entry.name.startsWith("account-")) continue;
    const resultPath = path.join(root, entry.name, "result.json");
    let result;
    try {
      result = JSON.parse(await fs.readFile(resultPath, "utf8"));
    } catch {
      continue;
    }
    const key = result.ok ? (result.email || result.userCode || result.signInId || result.runDir) : "";
      if (result.ok && key && !kept.has(key)) kept.add(key);
  }
  let nextIndex = 1;
  for (const entry of existing) {
    const match = /^account-(\d+)$/.exec(entry.name);
    if (match) nextIndex = Math.max(nextIndex, Number(match[1]) + 1);
  }
  for (const result of successes) {
    const key = result.email || result.userCode || result.signInId || result.runDir;
    if (kept.has(key)) continue;
    const runDir = path.join(root, `account-${String(nextIndex).padStart(3, "0")}`);
    nextIndex += 1;
    await fs.mkdir(runDir, { recursive: true });
    await writeJson(path.join(runDir, "result.json"), { ...result, runDir, importedAt: new Date().toISOString() });
    kept.add(key);
  }
  const summary = {
    batchId,
    requested: targetSuccessCount,
    completed: successes.length,
    done: successes.length,
    succeeded: successes.length,
    failed: Math.max(previousFailed, failures.filter((failure) => failure.batch === batchId).length),
    attempts: Math.max(previousAttempts, successes.length + failures.length),
    restartCount: previousRestartCount,
    updatedAt: new Date().toISOString(),
    finishedAt: successes.length >= targetSuccessCount ? new Date().toISOString() : null,
    results: [],
  };
  await writeJson(summaryPath, summary);
  return { succeeded: successes.length, failures: failures.length, summary };
}

async function runChild(restartCount, riskProbeCount, riskProbe = false) {
  return new Promise((resolve) => {
    const env = {
      ...process.env,
      BATCH_ROOT: batchRoot,
      BATCH_ID: batchId,
      TARGET_SUCCESS_COUNT: String(targetSuccessCount),
      MINIMAX_RESTART_COUNT: String(restartCount),
      MINIMAX_RISK_PROBE: riskProbe ? "1" : "0",
      MINIMAX_AUTOSTART: "",
      MINIMAX_AUTOSTART_DRY_RUN: "",
    };
    const child = spawn(process.execPath, [batchScript], { cwd: workspace, env, stdio: ["ignore", "pipe", "pipe"] });
    let stdoutTail = "";
    let stderrTail = "";
    let riskDetected = false;
    let captchaUnavailableDetected = false;
    const forward = (isError) => (chunk) => {
      const text = chunk.toString("utf8");
      let buffer = (isError ? stderrTail : stdoutTail) + text;
      const lines = buffer.split(/\r?\n/);
      buffer = lines.pop() || "";
      if (isError) stderrTail = buffer;
      else stdoutTail = buffer;
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        printStructuredProgress(trimmed, restartCount, riskProbeCount);
        if (isError) process.stderr.write(`${redactText(trimmed)}\n`);
      }
    };
    const forwardChunk = (isError) => (chunk) => {
      const text = chunk.toString("utf8");
      if (riskControlPattern.test(text)) riskDetected = true;
      if (captchaUnavailablePattern.test(text)) captchaUnavailableDetected = true;
      forward(isError)(text);
    };
    child.stdout.on("data", forwardChunk(false));
    child.stderr.on("data", forwardChunk(true));
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      clearInterval(progressTimer);
      resolve(value);
    };
    const progressTimer = setInterval(async () => {
      const ledger = await prepareResumeBatch();
      if (ledger.succeeded >= targetSuccessCount) {
        child.kill("SIGTERM");
        finish({ code: 0, error: "", targetReached: true, riskBlocked: false });
      }
    }, 2000);
    child.on("close", (code) => {
      if (stdoutTail.trim()) printStructuredProgress(stdoutTail.trim(), restartCount, riskProbeCount);
      if (stderrTail.trim()) process.stderr.write(`${redactText(stderrTail.trim())}\n`);
      if (riskControlPattern.test(stdoutTail) || riskControlPattern.test(stderrTail)) riskDetected = true;
      if (captchaUnavailablePattern.test(stdoutTail) || captchaUnavailablePattern.test(stderrTail)) captchaUnavailableDetected = true;
      const riskBlocked = riskDetected || code === riskExitCode;
      finish({ code, error: "", riskBlocked, captchaUnavailable: captchaUnavailableDetected });
    });
  });
}

function redactText(value) {
  return String(value)
    .replace(/(^|[?&])((?:k|key|token|password|secret|ticket|auth|authorization|code)=)[^&\s"'<>]+/gi, "$1$2***")
    .replace(/(PANEL_KEY|IMAP_PASSWORD|ticket|Proxy-Authorization|Authorization)([=:]\s*)[^\s,;]+/gi, "$1$2***")
    .replace(/(Bearer\s+)[A-Za-z0-9._~+/=-]+/gi, "$1***");
}

async function currentSummary() {
  const summary = await readJson(summaryPath, {});
  const succeeded = Number(summary.succeeded ?? summary.done ?? 0);
  const failed = Number(summary.failed ?? 0);
  const attempts = Number(summary.attempts ?? summary.completed ?? 0);
  return { succeeded, failed, attempts, finishedAt: summary.finishedAt || null };
}

function localClock(value) {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
}

async function readRiskState() {
  const stored = await readJson(riskStatePath, null);
  if (stored && Number.isFinite(Number(stored.cooldownUntil))) {
    if (stored.active === false) {
      await fs.rm(riskStatePath, { force: true });
      return null;
    }
    return { ...stored, active: Number(stored.cooldownUntil) > Date.now() };
  }
  const marker = await readJson(riskControlPath, null);
  if (!marker || !marker.at) return null;
  const cooldownUntil = new Date(marker.at).getTime() + riskCooldownSeconds * 1000;
  if (!Number.isFinite(cooldownUntil)) return null;
  return {
    cooldownUntil,
    cooldownSeconds: riskCooldownSeconds,
    reason: marker.error || "MiniMax 人机验证未通过",
    markerAt: marker.at,
    riskProbeCount: 0,
      active: false,
  };
}

function riskFields(state, summary, phase, message) {
  const remainingSeconds = Math.max(0, Math.ceil((state.cooldownUntil - Date.now()) / 1000));
  const supervisorRestartCount = Number(heartbeatFields.restartCount || 0);
  return {
    phase,
    running: true,
    succeeded: summary.succeeded,
    failed: summary.failed,
    restartCount: supervisorRestartCount,
    riskProbeCount: Number(state.riskProbeCount || 0),
    riskCooldownUntil: new Date(state.cooldownUntil).toISOString(),
    riskCooldownSeconds: remainingSeconds,
    lastExitCode: riskExitCode,
    message: message || `${state.reason || "MiniMax 人机验证未通过"}，冷却至 ${localClock(new Date(state.cooldownUntil))} 后单次探测；已保留 ${summary.succeeded} 个成功账号`,
  };
}

async function publishRiskCooldown(state, summary, message = "", restartCount = Number(heartbeatFields.restartCount || 0)) {
  const fields = riskFields(state, summary, "risk-cooldown", message);
  fields.restartCount = restartCount;
  await publishStatus(fields);
  await publishProgress(progressEvent({
    ...fields,
    done: summary.succeeded,
    ok: summary.succeeded,
    failed: summary.failed,
    attempts: summary.attempts,
    current: 0,
  }));
}

async function waitForRiskCooldown(state, restartCount) {
  while (Date.now() < state.cooldownUntil) {
    const summary = await currentSummary();
    if (summary.succeeded >= targetSuccessCount || summary.finishedAt) return true;
    await publishRiskCooldown(state, summary, "", restartCount);
    await delay(Math.min(2000, Math.max(100, state.cooldownUntil - Date.now())));
  }
  await fs.rm(riskControlPath, { force: true });
  return false;
}

async function startRiskCooldown(summary, reason, riskProbeCount, restartCount) {
  const markerAt = new Date().toISOString();
  const cooldownUntil = Date.now() + riskCooldownSeconds * 1000;
  const state = {
    cooldownUntil,
    cooldownSeconds: riskCooldownSeconds,
    reason,
    markerAt,
    riskProbeCount,
    active: true,
  };
  await writeJson(riskStatePath, state);
  await writeJson(riskControlPath, { at: markerAt, error: reason, probeOnly: true, runDir: root });
  await publishRiskCooldown(state, summary, "", restartCount);
  return state;
}

async function publishStatus(fields) {
  heartbeatFields = { ...heartbeatFields, ...fields };
  const previousSummary = await readJson(summaryPath, {});
  const heartbeatSucceeded = Number(previousSummary.succeeded || 0);
  const heartbeatFailed = Number(previousSummary.failed || 0);
  const status = {
    pid,
    batchId,
    targetSuccessCount,
    startedAt,
    heartbeatAt: new Date().toISOString(),
    ...heartbeatFields,
    succeeded: Math.max(Number(heartbeatFields.succeeded || 0), heartbeatSucceeded),
    failed: Math.max(Number(heartbeatFields.failed || 0), heartbeatFailed),
  };
  await writeJson(statusPath, status);
  const latestProgress = await readJson(progressPath, null);
  if (latestProgress && latestProgress.supervisorPid === pid && latestProgress.phase !== "done" && latestProgress.phase !== "failed") {
    await writeJson(progressPath, {
      ...latestProgress,
      done: Math.max(Number(latestProgress.done || 0), heartbeatSucceeded),
      ok: Math.max(Number(latestProgress.ok || 0), heartbeatSucceeded),
      failed: Math.max(Number(latestProgress.failed || 0), heartbeatFailed),
      restartCount: Number(heartbeatFields.restartCount || latestProgress.restartCount || 0),
      riskProbeCount: Number(heartbeatFields.riskProbeCount || latestProgress.riskProbeCount || 0),
      updatedAt: status.heartbeatAt,
      eventId: `${pid}-${startedAt}-heartbeat-${status.heartbeatAt}`,
    });
  }
}

async function main() {
  if (!Number.isFinite(targetSuccessCount) || targetSuccessCount < 1) throw new Error("TARGET_SUCCESS_COUNT must be at least 1");
  if (!(await acquireLock())) {
    const lock = await readJson(lockPath, {});
    await publishProgress(progressEvent({ message: `已有守护进程 ${lock.pid || "unknown"} 在运行`, phase: "locked" }));
    return;
  }
  let restartCount = 0;
  let riskProbeCount = 0;
  const heartbeatTimer = setInterval(() => {
    publishStatus({}).catch(() => {});
  }, 2000);
  heartbeatTimer.unref();
  try {
    let prepared = await prepareResumeBatch();
    await publishStatus({ phase: "resume", running: true, succeeded: prepared.succeeded, failedAttempts: prepared.failures, restartCount, message: `已恢复 ${prepared.succeeded}/${targetSuccessCount} 个成功账号` });
    await publishProgress(progressEvent({ phase: "resume", done: prepared.succeeded, ok: prepared.succeeded, attempts: prepared.succeeded, message: `已恢复 ${prepared.succeeded}/${targetSuccessCount} 个成功账号，正在继续补齐` }));
    while (true) {
      const risk = await readRiskState();
      if (risk) {
        const riskSummary = await currentSummary();
        if (riskSummary.succeeded >= targetSuccessCount || riskSummary.finishedAt) continue;
        if (risk.active === false && !risk.riskProbeCount) {
          await fs.rm(riskControlPath, { force: true });
          await startRiskCooldown(riskSummary, risk.reason, 0, restartCount);
          continue;
        }
        const targetReached = await waitForRiskCooldown(risk, restartCount);
        if (targetReached) continue;
        const probeSummary = await currentSummary();
        riskProbeCount = Number(risk.riskProbeCount || 0);
        await publishStatus({ phase: "risk-probe", running: true, succeeded: probeSummary.succeeded, failed: probeSummary.failed, restartCount, riskProbeCount, message: "风控冷却结束，正在执行单次注册探测" });
        await publishProgress(progressEvent({ phase: "risk-probe", done: probeSummary.succeeded, ok: probeSummary.succeeded, failed: probeSummary.failed, attempts: probeSummary.attempts, restartCount, riskProbeCount, message: "风控冷却结束，正在执行单次注册探测" }));
        riskProbeCount += 1;
        await writeJson(riskStatePath, { ...risk, cooldownUntil: Date.now(), active: false, riskProbeCount, lastProbeAt: new Date().toISOString() });
        const probe = await runChild(restartCount, riskProbeCount, true);
        const afterProbe = await currentSummary();
        if (afterProbe.succeeded >= targetSuccessCount || afterProbe.finishedAt) continue;
        if (probe.code === 0 && !probe.riskBlocked && !probe.captchaUnavailable) {
          await fs.rm(riskStatePath, { force: true });
          continue;
        }
        await startRiskCooldown(afterProbe, probe.captchaUnavailable ? "代理出口或 reCAPTCHA 票据环境暂不可用" : (probe.riskBlocked ? "MiniMax 人机验证未通过" : "单次探测未能完成注册"), riskProbeCount, restartCount);
        continue;
      }
      prepared = await prepareResumeBatch();
      const before = await currentSummary();
      if (prepared.succeeded >= targetSuccessCount || before.finishedAt) {
        await publishStatus({ phase: "done", running: false, succeeded: before.succeeded, failed: before.failed, restartCount, message: `已完成 ${before.succeeded}/${targetSuccessCount} 个成功账号` });
        await publishProgress(progressEvent({ phase: "done", done: before.succeeded, ok: before.succeeded, failed: before.failed, attempts: before.attempts, restartCount, message: `已完成 ${before.succeeded}/${targetSuccessCount} 个成功账号` }));
        return;
      }
      await publishStatus({ phase: "running", running: true, succeeded: before.succeeded, failed: before.failed, restartCount, riskProbeCount, message: restartCount ? `异常后第 ${restartCount} 次自动恢复，正在继续注册` : "正在注册账号" });
      if (restartCount) await publishProgress(progressEvent({ phase: "restart", done: before.succeeded, ok: before.succeeded, failed: before.failed, attempts: before.attempts, restartCount, message: `异常中断，第 ${restartCount} 次自动恢复，继续补齐 ${targetSuccessCount - before.succeeded} 个账号` }));
      const child = await runChild(restartCount, riskProbeCount);
      prepared = await prepareResumeBatch();
      const after = await currentSummary();
      if (child.riskBlocked || (!child.targetReached && child.code === 0)) {
        const riskReason = child.captchaUnavailable
          ? "代理出口或 reCAPTCHA 票据环境暂不可用"
          : (child.riskBlocked ? "MiniMax 人机验证未通过" : "批量流程提前结束");
        await startRiskCooldown(after, riskReason, riskProbeCount, restartCount);
        continue;
      }
      if (prepared.succeeded >= targetSuccessCount || after.finishedAt) {
        await publishStatus({ phase: "done", running: false, succeeded: after.succeeded, failed: after.failed, restartCount, message: `已完成 ${after.succeeded}/${targetSuccessCount} 个成功账号` });
        await publishProgress(progressEvent({ phase: "done", done: after.succeeded, ok: after.succeeded, failed: after.failed, attempts: after.attempts, restartCount, message: `已完成 ${after.succeeded}/${targetSuccessCount} 个成功账号` }));
        return;
      }
      restartCount += 1;
      await publishStatus({ phase: "restarting", running: true, succeeded: after.succeeded, failed: after.failed, restartCount, lastExitCode: child.code, message: `任务中断（退出码 ${child.code}），5 秒后自动恢复` });
      await publishProgress(progressEvent({ phase: "restarting", done: after.succeeded, ok: after.succeeded, failed: after.failed, attempts: after.attempts, restartCount, message: `任务中断（退出码 ${child.code}），5 秒后自动恢复` }));
      await delay(5000);
    }
  } finally {
    clearInterval(heartbeatTimer);
    await releaseLock();
  }
}

main().catch(async (error) => {
  await publishStatus({ phase: "failed", running: false, error: error.message, restartCount: 0 }).catch(() => {});
  console.error(JSON.stringify({ ok: false, error: error.message }));
  await releaseLock().catch(() => {});
  process.exitCode = 1;
});
