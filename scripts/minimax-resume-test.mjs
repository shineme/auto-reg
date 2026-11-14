import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const testRoot = path.join(workspace, "artifacts", `resume-unit-${crypto.randomBytes(4).toString("hex")}`);
const batchId = "resume-unit";
const supervisor = path.join(workspace, "scripts", "minimax-supervisor.mjs");

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

function result(email, runDir) {
  return {
    ok: true,
    email,
    user: `MiniMax${email.replace(/\D/g, "").slice(-6)}`,
    userCode: `TEST-${email.slice(-4)}`,
    signInId: `signin-${email}`,
    flowStatus: "approved",
    signInState: "done",
    runDir,
  };
}

async function main() {
  await fs.rm(testRoot, { recursive: true, force: true });
  await fs.mkdir(testRoot, { recursive: true });
  const oldBatch = path.join(testRoot, "20261114-000001");
  const oldDir = path.join(oldBatch, "account-001");
  await writeJson(path.join(oldDir, "result.json"), result("old@example.test", oldDir));

  const env = {
    ...process.env,
    BATCH_ROOT: testRoot,
    BATCH_ID: batchId,
    COUNT: "2",
    TARGET_SUCCESS_COUNT: "2",
    NODE_NO_WARNINGS: "1",
  };
  const first = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  const statusPath = path.join(testRoot, "supervisor.status.json");
  const summaryPath = path.join(testRoot, batchId, "summary.json");
  await waitFor(async () => (await readJson(statusPath, {}))?.pid === first.pid);
  first.kill("SIGTERM");
  await new Promise((resolve) => first.once("exit", resolve));
  await fs.rm(path.join(testRoot, "supervisor.lock"), { force: true });

  const resumedDir = path.join(testRoot, batchId, "account-002");
  await writeJson(path.join(resumedDir, "result.json"), result("resumed@example.test", resumedDir));

  const second = spawn(process.execPath, [supervisor], { cwd: workspace, env, stdio: "ignore" });
  await waitFor(async () => (await readJson(statusPath, {}))?.phase === "done");
  await new Promise((resolve) => second.once("exit", resolve));

  const ledger = await readJson(path.join(testRoot, "resume-ledger.json"));
  const summary = await readJson(summaryPath);
  const accountDirectories = (await fs.readdir(path.join(testRoot, batchId), { withFileTypes: true }))
    .filter((entry) => entry.isDirectory() && entry.name.startsWith("account-"))
    .map((entry) => entry.name);
  assert.equal(ledger.succeeded, 2);
  assert.equal(summary.succeeded, 2);
  assert.equal(summary.finishedAt ? "finished" : "unfinished", "finished");
  assert.deepEqual(accountDirectories.sort(), ["account-001", "account-002"]);
  assert.equal(await fs.stat(path.join(testRoot, "supervisor.lock")).then(() => true, () => false), false);
  console.log(JSON.stringify({ ok: true, testRoot, ledger: ledger.succeeded, summary: summary.succeeded, accountDirectories }));
  await fs.rm(testRoot, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(JSON.stringify({ ok: false, error: error.message, testRoot }));
  process.exitCode = 1;
});
