import fs from "node:fs";
import path from "node:path";

const root = "C:/Users/dhl/Desktop/m3/artifacts/gui-batch-200/resume-200";
const poolPath = "C:/Users/dhl/Desktop/m3/artifacts/gui-batch-200/hotmail-pool.json";
const pool = JSON.parse(fs.readFileSync(poolPath, "utf8"));
const counts = new Map();

for (const dir of fs.readdirSync(root, { withFileTypes: true }).filter((entry) => entry.isDirectory() && entry.name.startsWith("account-"))) {
  const resultPath = path.join(root, dir.name, "result.json");
  if (!fs.existsSync(resultPath)) continue;
  const result = JSON.parse(fs.readFileSync(resultPath, "utf8"));
  if (result.ok) continue;
  const email = String(result.email || "").toLowerCase();
  if (!email) continue;
  const error = String(result.error || "");
  const bad = /interrupted before result was saved|1200063|Authorization is not pending|grant is expired|AADSTS70000/i.test(error);
  if (!bad) continue;
  counts.set(email, (counts.get(email) || 0) + 1);
}

let changed = 0;
for (const account of pool.accounts) {
  const email = String(account.email || "").toLowerCase();
  const failCount = counts.get(email) || 0;
  if (failCount >= 1 && account.status === "available") {
    account.status = "skipped";
    account.failCount = Math.max(Number(account.failCount || 0), failCount);
    account.lastError = account.lastError || ("parked after " + failCount + " recent failures");
    account.lastCheckedAt = new Date().toISOString();
    changed += 1;
  }
}

fs.writeFileSync(poolPath, JSON.stringify(pool, null, 2));
const stats = {};
for (const account of pool.accounts) {
  stats[account.status] = (stats[account.status] || 0) + 1;
}
console.log(JSON.stringify({
  changed,
  stats,
  nextAvailable: pool.accounts.filter((account) => account.status === "available").slice(0, 8).map((account) => account.email),
}, null, 2));
