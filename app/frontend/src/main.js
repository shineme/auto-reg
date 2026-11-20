const go = window.go?.main?.App;
const runtime = window.runtime;
const form = document.getElementById("config-form");
const views = {
  setup: { title: "运行配置", subtitle: "配置域名邮箱、收信邮箱、代理与批量参数，然后启动协议链路。" },
  results: { title: "账号结果", subtitle: "检查最近一批账号是否通过 MiniMax 授权和 magpie 供应商绑定。" },
  logs: { title: "执行日志", subtitle: "实时查看协议流程输出、错误和账号完成事件。" },
};

let currentConfig = null;
let toastTimer = null;
const liveResults = new Map();

function element(id) {
  return document.getElementById(id);
}

function numberValue(name, fallback = 0) {
  const value = Number(form.elements[name].value);
  return Number.isFinite(value) ? value : fallback;
}

function readForm() {
  const data = new FormData(form);
  const config = {};
  for (const [key, value] of data.entries()) config[key] = value;
  for (const key of ["imapPort", "count", "hotmailPreflightWorkers"]) config[key] = numberValue(key);
  return config;
}

function fillForm(config) {
  currentConfig = config;
  for (const [key, value] of Object.entries(config)) {
    const input = form.elements[key];
    if (input) input.value = value ?? "";
  }
  updateMailMode();
  updateEmailPreview();
  updateProxyHint();
}

function currentMailMode() {
  const value = String(form.elements.mailMode?.value || "imap").toLowerCase();
  return value === "hotmail" || value === "hotmail-graph" ? "hotmail" : "imap";
}

function updateMailMode() {
  const mode = currentMailMode();
  if (form.elements.mailMode) form.elements.mailMode.value = mode;
  for (const node of document.querySelectorAll("[data-mail-mode]")) {
    node.hidden = node.dataset.mailMode !== mode;
  }
  const title = element("mail-panel-title");
  const subtitle = element("mail-panel-subtitle");
  if (title) title.textContent = mode === "hotmail" ? "Hotmail Graph 接码" : "接码邮箱 IMAP";
  if (subtitle) {
    subtitle.textContent = mode === "hotmail"
      ? "导入 email----password----clientId----refreshToken，测试通过后再批量注册。"
      : "授权码用于读取验证码，不会写入日志。";
  }
  if (mode === "hotmail") refreshHotmailStatus(false).catch(() => {});
}

function updateEmailPreview() {
  if (currentMailMode() !== "imap") return;
  const prefix = form.elements.mailPrefix.value.trim() || "mm";
  const domain = form.elements.mailDomain.value.trim() || "example.com";
  element("email-preview").textContent = `${prefix}<random>@${domain}`;
}

function normalizeProxyInput(raw) {
  let value = raw.trim().replace(/\\:/g, ":");
  const compact = value.match(/^([a-z][a-z0-9+.-]*):\/\/([^:/@?#]+):(.+):([^:/@?#]+):(\d+)([/?#].*)?$/i);
  if (!compact) return value;
  const userInfo = `${encodeURIComponent(compact[2])}:${encodeURIComponent(compact[3])}`;
  return `${compact[1]}://${userInfo}@${compact[4]}:${compact[5]}${compact[6] || ""}`;
}

function updateProxyHint() {
  const raw = form.elements.proxyUrl.value.trim();
  const hint = element("proxy-hint");
  if (!raw) {
    hint.textContent = "留空表示使用系统直连；批量运行时不会启动代理桥。";
    return;
  }
  const normalized = normalizeProxyInput(raw);
  let parsed;
  try { parsed = new URL(normalized); } catch {
    hint.textContent = "代理 URL 格式无效；支持 scheme://user:pass@host:port 或 scheme://user:pass:host:port。";
    return;
  }
  if (!["http:", "https:", "socks5:", "socks5h:"].includes(parsed.protocol) || !parsed.port) {
    hint.textContent = "仅支持 http/https/socks5/socks5h，且必须包含端口。";
    return;
  }
  hint.textContent = `已识别 ${parsed.protocol.replace(":", "").toUpperCase()} 代理：${parsed.hostname}:${parsed.port}`;
}

function toast(message, kind = "") {
  const node = element("toast");
  clearTimeout(toastTimer);
  node.className = `toast show ${kind}`;
  node.textContent = message;
  toastTimer = setTimeout(() => { node.className = "toast"; }, 3600);
}

function setView(name) {
  for (const button of document.querySelectorAll(".nav-item")) button.classList.toggle("active", button.dataset.view === name);
  for (const view of document.querySelectorAll(".view")) view.classList.toggle("active", view.id === `view-${name}`);
  element("view-title").textContent = views[name].title;
  element("view-subtitle").textContent = views[name].subtitle;
  if (name === "results") refreshResults();
}

function statusClass(status) {
  if (status.running) return "running";
  if (status.phase === "failed") return "failed";
  if (status.phase === "done") return "done";
  return "";
}

function renderStatus(status) {
  status = status || {};
  const pill = element("status-pill");
  pill.className = `status-pill ${statusClass(status)}`;
  const statusText = element("status-text");
  if (statusText) statusText.textContent = status.message || status.phase || "准备就绪";
  else pill.lastChild.textContent = status.message || status.phase || "准备就绪";
  const count = status.count || 0;
  const done = status.done || 0;
  const ok = status.ok || 0;
  const failed = status.failed || 0;
  const percent = count ? Math.min(100, (done / count) * 100) : 0;
  element("progress-fill").style.transform = `scaleX(${percent / 100})`;
  element("progress-label").textContent = status.message || "等待启动";
  element("progress-counter").textContent = `${done} / ${count}`;
  const details = [`成功 ${ok}`, `失败 ${failed}`, `尝试 ${status.attempts || 0}`];
  if (status.current) details.push(`当前第 ${status.current} 个`);
  if (status.restartCount) details.push(`自动恢复 ${status.restartCount} 次`);
  if (status.batchId) details.push(`批次 ${status.batchId}`);
  if (status.supervisorPid) details.push(`守护进程 ${status.supervisorPid}`);
  element("progress-detail").textContent = details.join(" · ");
  element("start-run").disabled = status.running;
  element("dry-run").disabled = status.running;
  element("test-panel").disabled = status.running;
  element("stop-run").disabled = !status.running;
  for (const id of ["import-hotmail", "test-hotmail", "refresh-hotmail", "purge-hotmail", "keep-hotmail"]) {
    const node = element(id);
    if (node) node.disabled = status.running;
  }
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--:--:--";
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

function appendLog(event) {
  if (!event) return;
  const list = element("log-list");
  const empty = list.querySelector(".empty");
  if (empty) empty.remove();
  const row = document.createElement("div");
  row.className = "log-row";
  const time = document.createElement("span");
  time.className = "log-time";
  time.textContent = formatTime(event.at);
  const level = document.createElement("span");
  level.className = `log-level ${event.level || "info"}`;
  level.textContent = event.level || "info";
  const message = document.createElement("span");
  message.className = "log-message";
  message.textContent = event.message || "";
  row.append(time, level, message);
  list.append(row);
  while (list.children.length > 400) list.firstChild.remove();
  list.scrollTop = list.scrollHeight;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[character]);
}

function renderResults(results) {
  const body = element("results-body");
  body.innerHTML = "";
  results = Array.isArray(results) ? results : [];
  if (!results.length) {
    body.innerHTML = '<tr><td colspan="5" class="empty">暂无结果；完成一次运行后会显示在这里。</td></tr>';
    return;
  }
  for (const result of results) {
    const row = document.createElement("tr");
    const badge = result.ok ? '<span class="badge ok">成功</span>' : '<span class="badge fail">失败</span>';
    row.innerHTML = `<td>${badge}</td><td>${escapeHtml(result.email || "")}</td><td>${escapeHtml(result.user || "")}</td><td>${escapeHtml(result.userCode || "")}</td><td>${escapeHtml(result.error || "")}</td>`;
    body.append(row);
  }
}

async function refreshResults() {
  if (!go) return;
  try {
    const response = await go.GetLatestResults();
    const results = Array.isArray(response) ? response : (response?.results || []);
    for (const result of results) {
      const key = result.email || result.userCode || result.signInId || result.runDir;
      if (key) liveResults.set(key, result);
    }
    renderResults([...liveResults.values()]);
  } catch (error) { toast(`读取结果失败：${error.message || error}`, "error"); }
}

async function checkRuntime() {
  if (!go) return;
  try {
    const result = await go.CheckRuntime();
    const set = (id, value) => {
      const node = element(id);
      node.textContent = value;
      node.className = value === "missing" ? "missing" : (value === "ok" || value.startsWith("v") ? "ok" : "");
    };
    set("runtime-node", result.node || "missing");
    set("runtime-chrome", result.chrome || "missing");
    set("runtime-scripts", result.scripts || "missing");
  } catch (error) { toast(`运行环境检查失败：${error.message || error}`, "error"); }
}

async function saveConfig(showMessage = true) {
  const config = readForm();
  await go.ValidateConfig(config);
  await go.SaveConfig(config);
  currentConfig = config;
  if (showMessage) toast("配置已保存到本机", "success");
  return config;
}

async function startRun(dryRun) {
  try {
    const config = await saveConfig(false);
    await go.StartRun(config, dryRun);
    toast(dryRun ? "票据 dry-run 已启动" : "批量注册已启动", "success");
  } catch (error) { toast(`无法启动：${error.message || error}`, "error"); }
}

async function testPanel() {
  const button = element("test-panel");
  button.disabled = true;
  try {
    const result = await go.TestPanel(readForm());
    toast(result.message || "面板连接正常", "success");
  } catch (error) { toast(`面板检查失败：${error.message || error}`, "error"); }
  finally { button.disabled = false; }
}

async function importHotmail(replace = false) {
  const button = element("import-hotmail");
  button.disabled = true;
  try {
    const result = await go.ImportHotmailAccounts(readForm(), replace);
    toast(result.message || "Hotmail 账号已导入", "success");
    if (result.filePath && form.elements.hotmailPool) form.elements.hotmailPool.value = result.filePath;
    await refreshHotmailStatus(false);
  } catch (error) {
    toast(`导入失败：${error.message || error}`, "error");
  } finally {
    button.disabled = false;
  }
}

function updateHotmailProgress(payload = {}) {
  const wrap = element("hotmail-progress");
  if (!wrap) return;
  wrap.hidden = false;
  const current = Number(payload.current || 0);
  const total = Number(payload.total || 0);
  const percent = total > 0 ? Math.max(0, Math.min(100, (current / total) * 100)) : 0;
  const fill = element("hotmail-progress-fill");
  if (fill) fill.style.transform = `scaleX(${percent / 100})`;
  const label = element("hotmail-progress-label");
  if (label) label.textContent = payload.message || "正在测活";
  const counter = element("hotmail-progress-counter");
  if (counter) counter.textContent = `${current} / ${total}`;
  const detail = element("hotmail-progress-detail");
  if (detail) {
    const parts = [];
    if (payload.email) parts.push(payload.email);
    if (typeof payload.okCount === "number") parts.push(`可用 ${payload.okCount}`);
    if (typeof payload.errorCount === "number") parts.push(`失效 ${payload.errorCount}`);
    if (typeof payload.workers === "number" && payload.workers > 0) parts.push(`并行 ${payload.workers}`);
    if (typeof payload.inFlight === "number" && payload.inFlight > 0) parts.push(`进行中 ${payload.inFlight}`);
    detail.textContent = parts.join(" · ") || "测活时会显示实时进度";
  }
}

function setHotmailBusy(busy) {
  for (const id of ["import-hotmail", "test-hotmail", "refresh-hotmail", "purge-hotmail", "keep-hotmail"]) {
    const node = element(id);
    if (node) node.disabled = !!busy;
  }
}

async function testHotmail() {
  const button = element("test-hotmail");
  button.disabled = true;
  setHotmailBusy(true);
  updateHotmailProgress({ current: 0, total: 0, message: "正在准备批量测活" });
  try {
    const result = await go.PreflightHotmailAccounts(readForm(), 0);
    updateHotmailProgress({
      current: result.checked || 0,
      total: result.checked || 0,
      okCount: result.okCount || 0,
      errorCount: result.errorCount || 0,
      message: result.message || "测活完成",
    });
    toast(result.message || "测活完成", "success");
    await refreshHotmailStatus(false);
  } catch (error) {
    toast(`测活失败：${error.message || error}`, "error");
  } finally {
    setHotmailBusy(false);
  }
}

async function purgeInvalidHotmail() {
  if (!window.confirm("将从账号池删除所有已标记失效的邮箱，确认继续？")) return;
  setHotmailBusy(true);
  try {
    const result = await go.PurgeInvalidHotmailAccounts(readForm());
    toast(result.message || "已移除失效账号", "success");
    await refreshHotmailStatus(false);
  } catch (error) {
    toast(`移除失效失败：${error.message || error}`, "error");
  } finally {
    setHotmailBusy(false);
  }
}

async function keepValidHotmail() {
  if (!window.confirm("将只保留可用和已成功使用的邮箱，失效/占用中的都会删除。确认继续？")) return;
  setHotmailBusy(true);
  try {
    const result = await go.KeepValidHotmailAccounts(readForm());
    toast(result.message || "已仅保留有效账号", "success");
    await refreshHotmailStatus(false);
  } catch (error) {
    toast(`保留有效失败：${error.message || error}`, "error");
  } finally {
    setHotmailBusy(false);
  }
}

async function refreshHotmailStatus(showToast = true) {
  if (!go || currentMailMode() !== "hotmail") return;
  try {
    const result = await go.GetHotmailPoolStatus(readForm());
    const hint = element("hotmail-hint");
    if (hint) {
      hint.textContent = result.message || `可用 ${result.available || 0} · 失效 ${result.error || 0} · 已用 ${result.used || 0} · 总计 ${result.total || 0}`;
    }
    if (result.filePath && form.elements.hotmailPool) form.elements.hotmailPool.value = result.filePath;
    if (showToast) toast(result.message || "账号池状态已刷新", "success");
  } catch (error) {
    if (showToast) toast(`刷新池状态失败：${error.message || error}`, "error");
  }
}

async function clearRunState(includeHistory) {
  if (!go) return;
  const question = includeHistory
    ? "将删除本机输出目录中的历史账号结果、汇总、风控和守护状态；真实面板账号不会被删除。确认继续？"
    : "将删除本机旧风控标记、守护状态和运行状态；已成功账号记录会保留。确认继续？";
  if (!window.confirm(question)) return;
  try {
    const result = await go.ClearRunState(includeHistory);
    toast(includeHistory ? "历史记录已清空，可以重新开始" : "运行状态已清理，可以重新点击开始", "success");
    liveResults.clear();
    renderResults([]);
    renderStatus(await go.GetRunStatus());
    if (result?.includeHistory) await refreshResults();
  } catch (error) { toast(`清理失败：${error.message || error}`, "error"); }
}

async function init() {
  if (!go) {
    toast("Wails 绑定未就绪，请用 wails dev 或打包后的程序启动。", "error");
    return;
  }
  for (const button of document.querySelectorAll(".nav-item")) button.addEventListener("click", () => setView(button.dataset.view));
  for (const name of ["mailPrefix", "mailDomain"]) form.elements[name].addEventListener("input", updateEmailPreview);
  form.elements.mailMode.addEventListener("change", () => {
    updateMailMode();
    updateEmailPreview();
  });
  form.elements.proxyUrl.addEventListener("input", updateProxyHint);
  element("save-config").addEventListener("click", () => saveConfig().catch((error) => toast(`保存失败：${error.message || error}`, "error")));
  element("test-panel").addEventListener("click", () => testPanel());
  element("import-hotmail").addEventListener("click", () => importHotmail(false));
  element("test-hotmail").addEventListener("click", () => testHotmail());
  element("refresh-hotmail").addEventListener("click", () => refreshHotmailStatus(true));
  element("purge-hotmail").addEventListener("click", () => purgeInvalidHotmail());
  element("keep-hotmail").addEventListener("click", () => keepValidHotmail());
  element("dry-run").addEventListener("click", () => startRun(true));
  element("start-run").addEventListener("click", () => startRun(false));
  element("stop-run").addEventListener("click", () => go.StopRun());
  element("clear-state").addEventListener("click", () => clearRunState(false));
  element("clear-history").addEventListener("click", () => clearRunState(true));
  element("refresh-results").addEventListener("click", refreshResults);
  element("check-runtime").addEventListener("click", checkRuntime);
  element("open-output").addEventListener("click", async () => {
    try { await go.OpenOutputDir(); } catch (error) { toast(`打开目录失败：${error.message || error}`, "error"); }
  });
  element("clear-logs").addEventListener("click", () => { element("log-list").innerHTML = '<div class="empty">已清空显示；后台日志仍保留在会话中。</div>'; });
  form.addEventListener("submit", (event) => event.preventDefault());

  runtime.EventsOn("hotmail-progress", (payload) => updateHotmailProgress(payload || {}));
  runtime.EventsOn("run-event", (event) => {
    event = event || {};
    appendLog(event);
    if (event.type === "result" && event.result) {
      const key = event.result.email || event.result.userCode || event.result.signInId || event.result.runDir;
      if (key) liveResults.set(key, event.result);
      if (document.getElementById("view-results").classList.contains("active")) renderResults([...liveResults.values()]);
    }
    if (event.type === "refresh-results") refreshResults();
  });
  runtime.EventsOn("run-status", (status) => {
    status = status || {};
    renderStatus(status);
    if (!status.running && ["done", "failed", "stopped", "domain-blocked", "environment-unavailable"].includes(status.phase)) refreshResults();
  });

  try {
    fillForm((await go.GetConfig()) || {});
    renderStatus((await go.GetRunStatus()) || {});
    const response = await go.GetRunEvents();
    const events = Array.isArray(response) ? response : (response?.events || []);
    if (events.length) { element("log-list").innerHTML = ""; events.forEach(appendLog); }
    await refreshResults();
  } catch (error) { toast(`加载界面数据失败：${error.message || error}`, "error"); }
  await checkRuntime();
}

window.addEventListener("DOMContentLoaded", init);


