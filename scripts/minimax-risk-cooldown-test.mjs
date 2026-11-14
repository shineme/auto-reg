import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const testRoot = path.join(workspace, "artifacts", `risk-unit-${crypto.randomBytes(4).toString("hex")}`);
const batchId = "risk-unit";
const supervisor = path.join(workspace, "scripts", "minimax-supervisor.mjs");
const batchScript = path.join(testRoot, "fake-batch.mjs");
const statusPath = path.join(testRoot, "supervisor.status.json");
const progressPath = path.join(testRoot, "supervisor.progress.json");
const summaryPath = path.join(testRoot, batchId, "summary.json");
const riskStatePath = path.join(testRoot, "risk-state.json");
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
const count = Number(await fs.readFile(counterPath, "utf8").catch(() => "0")) + 1;
await fs.writeFile(counterPath, String(count));
if (count === 1) {
  console.error("send-code: 1200063 Verification failed, please try again.");
  process.exitCode = 75;
} else if (count === 2) {
  console.log(JSON.stringify({ type: "progress", phase: "risk-recovered", message: "单次探测注册成功", done: 1, ok: 1, failed: 0, attempts: 1, current: 0, updatedAt: new Date().toISOString() }));
  await fs.mkdir(${JSON.stringify(path.join(testRoot, batchId, "account-002"))}, { recursive: true });
  await fs.writeFile(${JSON.stringify(path.join(testRoot, batchId, "account-002", "result.json"))}, JSON.stringify({ ok: true, email: "recovered@example.test", user: "MiniMax100002", runDir: ${JSON.stringify(path.join(testRoot, batchId, "account-002"))} }));
  process.exitCode = 0;
} else {
  console.error(JSON.stringify({ ok: false, error: "unexpected fake child invocation" }));
  process.exitCode = 1;
}
`, "utf8");
  const env = {
    ...process.env,
    BATCH_ROOT: testRoot,
    BATCH_ID: batchId,
    COUNT: "2",
    TARGET_SUCCESS_COUNT: "2",
    MINIMAX_BATCH_SCRIPT: batchScript,
    MINIMAX_RISK_COOLDOWN_SECONDS: "2",
    NODE_NO_WARNINGS: "1",
  };
  const child = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  await waitFor(async () => (await readJson(statusPath, {}))?.phase === "risk-cooldown");
  const cooldownStatus = await readJson(statusPath, {});
  assert.equal(cooldownStatus.running, true);
  assert.equal(cooldownStatus.lastExitCode, 75);
  assert.equal(cooldownStatus.succeeded, 1);
  assert.ok(cooldownStatus.riskCooldownUntil);
  assert.match(cooldownStatus.message, /冷却至/);
  const cooldownProgress = await readJson(progressPath, {});
  assert.equal(cooldownProgress.phase, "risk-cooldown");
  assert.equal(cooldownProgress.done, 1);
  const riskState = await readJson(riskStatePath, {});
  assert.equal(riskState.active, true);
  assert.equal(await childCount(), 1);
  await waitFor(async () => (await readJson(statusPath, {}))?.phase === "done", 20000);
  await new Promise((resolve) => child.once("exit", resolve));
  const doneStatus = await readJson(statusPath, {});
  const summary = await readJson(summaryPath, {});
  const ledger = await readJson(path.join(testRoot, "resume-ledger.json"), {});
  assert.equal(doneStatus.running, false);
  assert.equal(doneStatus.succeeded, 2);
  assert.equal(doneStatus.restartCount, 0);
  assert.equal(summary.succeeded, 2);
  assert.equal(summary.finishedAt ? "finished" : "unfinished", "finished");
  assert.equal(ledger.succeeded, 2);
  assert.equal(await childCount(), 2);
  assert.equal(await fs.stat(path.join(testRoot, "supervisor.lock")).then(() => true, () => false), false);
  console.log(JSON.stringify({ ok: true, testRoot, cooldownPhase: cooldownStatus.phase, probes: await childCount(), succeeded: summary.succeeded }));
  await fs.rm(testRoot, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message, testRoot }));
  process.exitCode = 1;
});
