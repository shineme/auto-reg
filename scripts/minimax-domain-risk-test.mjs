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

async function runCase(name, expectedRuns, staleMarker) {
  await fs.rm(testRoot, { recursive: true, force: true });
  await fs.mkdir(testRoot, { recursive: true });
  await writeJson(path.join(testRoot, batchId, "account-001", "result.json"), {
    ok: true,
    email: "existing@example.test",
    user: "MiniMax100001",
    runDir: path.join(testRoot, batchId, "account-001"),
  });
  if (staleMarker) await writeJson(path.join(testRoot, batchId, "risk-control.json"), { reason: "domain-risk-controlled", mailDomain: "old.example.test" });
  await fs.writeFile(batchScript, `
import fs from "node:fs/promises";
const counterPath = ${JSON.stringify(counterPath)};
const count = Number(await fs.readFile(counterPath, "utf8").catch(() => "0")) + 1;
await fs.writeFile(counterPath, String(count));
console.error("send-code: 1200063 Verification failed, please try again.");
if (count < ${expectedRuns}) process.exitCode = 1;
else console.log(JSON.stringify({ type: "summary", summary: { succeeded: 1 } }));
`, "utf8");
  const env = {
    ...process.env,
    BATCH_ROOT: testRoot,
    BATCH_ID: batchId,
    COUNT: "2",
    TARGET_SUCCESS_COUNT: "2",
    MINIMAX_BATCH_SCRIPT: batchScript,
    MINIMAX_RETRY_DELAY_MS: "50",
    NODE_NO_WARNINGS: "1",
  };
  const child = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  await waitFor(async () => Number(await fs.readFile(counterPath, "utf8").catch(() => "0")) >= expectedRuns, 15000);
  await new Promise((resolve) => child.once("exit", resolve));
  const status = await readJson(statusPath, {});
  const progress = await readJson(progressPath, {});
  assert.equal(await childCount(), expectedRuns);
  assert.notEqual(status.phase, "domain-blocked", "verification failures must not be treated as domain risk");
  assert.ok(!/域名已被风控/.test(status.message || ""), "domain-risk message must not appear");
  assert.equal(status.restartCount, expectedRuns - 1);
  assert.equal(await fs.stat(path.join(testRoot, "supervisor.lock")).then(() => true, () => false), false);
  if (staleMarker) assert.equal(await fs.stat(path.join(testRoot, batchId, "risk-control.json")).then(() => true, () => false), false);
  console.log(JSON.stringify({ ok: true, case: name, childRuns: expectedRuns, phase: status.phase, staleMarkerIgnored: Boolean(staleMarker), restartCount: status.restartCount }));
}

async function main() {
  await runCase("verification-failure-retries", 2, false);
  await runCase("stale-risk-marker-ignored", 2, true);
  await fs.rm(testRoot, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message, testRoot }));
  process.exitCode = 1;
});
