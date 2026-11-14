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
  for (const key of ["imapPort", "count"]) config[key] = numberValue(key);
  return config;
}

function fillForm(config) {
  currentConfig = config;
  for (const [key, value] of Object.entries(config)) {
    const input = form.elements[key];
    if (input) input.value = value ?? "";
  }
  updateEmailPreview();
  updateProxyHint();
}

function updateEmailPreview() {
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
  const pill = element("status-pill");
  pill.className = `status-pill ${statusClass(status)}`;
  pill.lastChild.textContent = status.message || status.phase || "准备就绪";
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
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "--:--:--";
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

function appendLog(event) {
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
    const results = await go.GetLatestResults();
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

async function init() {
  if (!go) {
    toast("Wails 绑定未就绪，请用 wails dev 或打包后的程序启动。", "error");
    return;
  }
  for (const button of document.querySelectorAll(".nav-item")) button.addEventListener("click", () => setView(button.dataset.view));
  for (const name of ["mailPrefix", "mailDomain"]) form.elements[name].addEventListener("input", updateEmailPreview);
  form.elements.proxyUrl.addEventListener("input", updateProxyHint);
  element("save-config").addEventListener("click", () => saveConfig().catch((error) => toast(`保存失败：${error.message || error}`, "error")));
  element("test-panel").addEventListener("click", () => testPanel());
  element("dry-run").addEventListener("click", () => startRun(true));
  element("start-run").addEventListener("click", () => startRun(false));
  element("stop-run").addEventListener("click", () => go.StopRun());
  element("refresh-results").addEventListener("click", refreshResults);
  element("check-runtime").addEventListener("click", checkRuntime);
  element("open-output").addEventListener("click", async () => {
    try { await go.OpenOutputDir(); } catch (error) { toast(`打开目录失败：${error.message || error}`, "error"); }
  });
  element("clear-logs").addEventListener("click", () => { element("log-list").innerHTML = '<div class="empty">已清空显示；后台日志仍保留在会话中。</div>'; });
  form.addEventListener("submit", (event) => event.preventDefault());

  runtime.EventsOn("run-event", (event) => {
    appendLog(event);
    if (event.type === "result" && event.result) {
      const key = event.result.email || event.result.userCode || event.result.signInId || event.result.runDir;
      if (key) liveResults.set(key, event.result);
      if (document.getElementById("view-results").classList.contains("active")) renderResults([...liveResults.values()]);
    }
    if (event.type === "refresh-results") refreshResults();
  });
  runtime.EventsOn("run-status", (status) => {
    renderStatus(status);
    if (!status.running && (status.phase === "done" || status.phase === "failed")) refreshResults();
  });

  try {
    fillForm(await go.GetConfig());
    renderStatus(await go.GetRunStatus());
    const events = await go.GetRunEvents();
    if (events.length) { element("log-list").innerHTML = ""; events.forEach(appendLog); }
    await refreshResults();
  } catch (error) { toast(`加载配置失败：${error.message || error}`, "error"); }
  await checkRuntime();
}

window.addEventListener("DOMContentLoaded", init);
