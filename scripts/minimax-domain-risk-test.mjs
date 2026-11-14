import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const testRoot = path.join(workspace, "artifacts", `domain-risk-unit-${crypto.randomBytes(4).toString("hex")}`);
const batchId = "risk-unit";
const supervisor = path.join(workspace, "scripts", "minimax-supervisor.mjs");
const batchScript = path.join(testRoot, "fake-batch.mjs");
const statusPath = path.join(testRoot, "supervisor.status.json");
const progressPath = path.join(testRoot, "supervisor.progress.json");
const summaryPath = path.join(testRoot, batchId, "summary.json");
const riskStatePath = path.join(testRoot, "risk-state.json");
const riskControlPath = path.join(testRoot, batchId, "risk-control.json");
const counterPath = path.join(testRoot, "child-count.txt");

async function writeJson(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, JSON.stringify(value, null, 2), "utf8");
}

async function readJson(filePath, fallback = null) {
  try {
    return JSON.parse(await fs.readFile(filePath, "utf8"));
  } catch {
    return fallback;
  }
}

async function waitFor(predicate, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("waitFor timed out");
}

async function childCount() {
  return Number(await fs.readFile(counterPath, "utf8").catch(() => "0"));
}

async function main() {
  await fs.rm(testRoot, { recursive: true, force: true });
  await fs.mkdir(testRoot, { recursive: true });
  await writeJson(path.join(testRoot, batchId, "account-001", "result.json"), {
    ok: true,
    email: "existing@example.test",
    user: "MiniMax100001",
    runDir: path.join(testRoot, batchId, "account-001"),
  });
  await fs.writeFile(batchScript, `
import fs from "node:fs/promises";
const counterPath = ${JSON.stringify(counterPath)};
const riskControlPath = ${JSON.stringify(riskControlPath)};
const count = Number(await fs.readFile(counterPath, "utf8").catch(() => "0")) + 1;
await fs.writeFile(counterPath, String(count));
await fs.mkdir(${JSON.stringify(path.join(testRoot, batchId))}, { recursive: true });
await fs.writeFile(riskControlPath, JSON.stringify({ at: new Date().toISOString(), reason: "domain-risk-controlled", mailDomain: "blocked.example.test", error: "1200063" }, null, 2));
console.error("send-code: 1200063 Verification failed, please try again.");
process.exitCode = 75;
`, "utf8");
  const env = {
    ...process.env,
    BATCH_ROOT: testRoot,
    BATCH_ID: batchId,
    COUNT: "2",
    TARGET_SUCCESS_COUNT: "2",
    MAIL_DOMAIN: "fresh.example.test",
    MINIMAX_BATCH_SCRIPT: batchScript,
    NODE_NO_WARNINGS: "1",
  };
  const child = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  await waitFor(async () => (await readJson(statusPath, {}))?.phase === "domain-blocked", 15000);
  await new Promise((resolve) => child.once("exit", resolve));
  const status = await readJson(statusPath, {});
  const progress = await readJson(progressPath, {});
  const summary = await readJson(summaryPath, {});
  assert.equal(status.running, false);
  assert.equal(status.lastExitCode, 75);
  assert.equal(status.succeeded, 1);
  assert.equal(status.restartCount, 0);
  assert.match(status.message, /域名已被风控/);
  assert.equal(progress.phase, "domain-blocked");
  assert.equal(summary.succeeded, 1);
  assert.equal(await childCount(), 1);
  assert.equal(await fs.stat(riskStatePath).then(() => true, () => false), false);
  assert.equal(await fs.stat(path.join(testRoot, "supervisor.lock")).then(() => true, () => false), false);

  const beforeRestart = await childCount();
  await writeJson(riskControlPath, { at: new Date().toISOString(), reason: "domain-risk-controlled", mailDomain: "fresh.example.test", error: "1200063" });
  const resumed = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  await waitFor(async () => (await readJson(statusPath, {}))?.phase === "domain-blocked", 15000);
  await new Promise((resolve) => resumed.once("exit", resolve));
  assert.equal(await childCount(), beforeRestart);
  assert.equal((await readJson(statusPath, {})).running, false);
  console.log(JSON.stringify({ ok: true, phase: status.phase, childRuns: 1, oldRiskStateRemoved: true, restartCount: status.restartCount }));
  await fs.rm(testRoot, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message, testRoot }));
  process.exitCode = 1;
});
