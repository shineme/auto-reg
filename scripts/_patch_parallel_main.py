from pathlib import Path
bak = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs.bak-parallel').read_text(encoding='utf-8')
cur = Path(r'C:\Users\dhl\Desktop\m3\scripts\minimax-fast-batch.mjs')
src = cur.read_text(encoding='utf-8')

# extract original main from backup
b_start = bak.find('async function main()')
b_end = bak.find('main().catch')
original_main = bak[b_start:b_end]
# Keep the preamble of main until the while loop, then replace loop with parallel version.
marker = '  emitProgress("resume"'
pos = original_main.find(marker)
if pos < 0:
    raise SystemExit('resume marker missing')
# find end of that emitProgress line
line_end = original_main.find('\n', pos)
preamble = original_main[:line_end+1]
# The Chinese emitProgress line may be after updateProgress/writeSummary already included.
# Better cut after writeSummary/emitProgress resume.
cut = original_main.find('  while (progressState.succeeded < targetSuccessCount)')
if cut < 0:
    raise SystemExit('while missing')
preamble = original_main[:cut]

parallel_loop = r'''  emitProgress("workers", `注册并行 ${workers} 路启动`);
  let nextIndexGate = Promise.resolve();
  async function reserveNextIndex() {
    let assigned = 0;
    const waiter = nextIndexGate.then(() => {
      assigned = nextIndex;
      nextIndex += 1;
      progressState.current = assigned;
      return assigned;
    });
    nextIndexGate = waiter.then(() => {}, () => {});
    return waiter;
  }

  async function runOneAccount() {
    const index = await reserveNextIndex();
    let bridge = { proxyUrl, cleanup: async () => {} };
    let task;
    let result;
    try {
      bridge = await createStickyProxyBridge(index);
      emitProgress("proxy-rotate", `已分配独立代理会话：准备第 ${index} 个账号`, { current: index, workers });
      emitProgress("creating-code", `正在创建第 ${index} 个账号设备码`, { current: index, workers });
      task = await createTask(index);
      task.proxyUrl = bridge.proxyUrl;
      await initializeTask(task);
      const beforeUsers = (await providerUsers()).users;
      emitProgress("captcha", `正在为 ${task.email} 创建独立临时 Chrome profile 并预热 reCAPTCHA 票据`, { current: task.index, workers });
      try {
        await withCdpBrowser(task, async (cdp, browser) => {
          task.browserProfileDir = browser.profileDir;
          task.browserCdpPort = browser.port;
          task.browserChromePid = browser.child.pid;
          task.context = await createAccountContext(cdp, task);
          result = await processAccount(task, beforeUsers, (step, message) => emitProgress(step, message, { current: task.index, workers }));
          await closeBrowserContext(cdp, task.context);
        }, bridge.proxyUrl);
      } catch (error) {
        if (!result) {
          task.prewarmError = error.message;
          result = {
            index: task.index,
            ok: false,
            error: task.prewarmError,
            runDir: task.runDir,
          };
          await writeJson(task.resultPath, {
            ok: false,
            error: task.prewarmError,
            email: task.email,
            userCode: task.userCode,
            signInId: task.signInId,
            runDir: task.runDir,
            completedAt: new Date().toISOString(),
          });
          await finalizeHotmailAccount(task, { ...result, email: task.email, userCode: task.userCode, signInId: task.signInId });
        }
      }
      await enqueueState(async () => {
        results.push(result);
        updateProgressState(results, nextIndex);
        await writeSummary(results);
      });
      emitAccountResult(result);
      if (captchaUnavailablePattern.test(result.error || "")) {
        emitProgress("captcha-retry", "本轮 reCAPTCHA 票据环境异常，守护进程将自动恢复", { current: task.index, workers });
        process.exitCode = 1;
        throw new Error(result.error || "reCAPTCHA Enterprise did not become ready");
      }
      emitProgress("account-done", result.ok ? `账号注册成功：${result.user || result.email}` : `账号未成功：${result.error || "未知错误"}`, { current: task.index, workers });
      if (!result.ok) {
        const waitMs = failureBackoffMs(result.error || "");
        if (waitMs > 0) {
          emitProgress("backoff", `失败后冷却 ${Math.round(waitMs / 1000)} 秒再继续，避免触发限流/人机风控`, { current: task.index, workers });
          await delay(waitMs);
        }
      } else {
        const gapMs = Math.max(0, Number(process.env.MINIMAX_ACCOUNT_GAP_MS || 1500));
        if (gapMs > 0) await delay(gapMs);
      }
    } finally {
      try { await bridge.cleanup(); } catch {}
    }
  }

  const active = new Set();
  let fatal = null;
  while (!fatal && progressState.succeeded < targetSuccessCount) {
    while (!fatal && progressState.succeeded + active.size < targetSuccessCount && active.size < workers) {
      const job = runOneAccount()
        .catch((error) => {
          fatal = error;
          process.exitCode = 1;
        })
        .finally(() => active.delete(job));
      active.add(job);
    }
    if (!active.size) break;
    await Promise.race(active);
  }
  if (active.size) await Promise.allSettled([...active]);
  if (fatal) throw fatal;
  const summary = await writeSummary(results, true);
  console.log(JSON.stringify({ type: "summary", summary, summaryPath }));
}

'''

new_main = preamble + parallel_loop + '\n'

# replace main in current file
c_start = src.find('async function main()')
c_end = src.find('main().catch')
if c_start < 0 or c_end < 0:
    raise SystemExit('current main markers missing')
src = src[:c_start] + new_main + src[c_end:]

# patch taskEnv to use task.proxyUrl
old_env = 'MINIMAX_PROXY_URL: proxyUrl,'
new_env = 'MINIMAX_PROXY_URL: task.proxyUrl || proxyUrl,'
if old_env not in src:
    raise SystemExit('taskEnv proxy missing')
src = src.replace(old_env, new_env, 1)

cur.write_text(src, encoding='utf-8')
print('main parallelized')
# verify chinese not garbled in new emit strings
print('ok strings', '正在创建第' in src, '账号注册成功' in src)
