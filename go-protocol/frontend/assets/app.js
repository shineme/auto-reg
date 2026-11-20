/* MiniMax 协议注册工作台 — 前端逻辑 */
(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const app = () => window.go.main.App;

  const FIELDS = [
    "panelBase", "panelKey", "panel2Base", "panel2Key", "mailMode", "count", "workers",
    "captchaDirect", "mailViaProxy", "signinAfterRegister", "timezoneId",
    "mailDomain", "mailPrefix", "imapHost", "imapPort", "imapUser", "imapPassword",
    "hotmailRaw", "hotmailPool", "hotmailPreflightWorkers",
    "proxyUrl", "proxyRegionRotate", "proxySessTime", "proxyStyle", "proxyState", "proxySticky",
    "proxySubscriptionUrl", "proxyPoolFile", "proxyPoolUse", "proxyPoolWorkers", "proxyLocalPorts",
    "fixedPassword", "ticketMode", "externalTicketUrl",
    "signatureSalt", "yySuffix", "recaptchaSiteKey", "chromePath", "outputDir"
  ];

  const STAGE_ICON = {
    "申请设备码": "▣",
    "获取风控票据": "◈",
    "注册设备": "◉",
    "检查账号状态": "◇",
    "发送邮箱验证码": "✉",
    "轮询邮箱验证码": "↻",
    "创建密码并登录": "◆",
    "设备授权审批": "✓",
    "等待面板绑定完成": "◎",
    "每日签到领取积分": "★"
  };
  const STAGE_KIND = {
    "轮询邮箱验证码": "warn",
    "等待面板绑定完成": "warn"
  };

  const state = {
    cfg: null,
    saved: null,
    dirty: false,
    summary: { succeeded: 0, target: 0, failedAttempts: 0, running: false, points: 0 },
    items: [],
    itemSeq: 0,
    logFilter: "all",
    acc: { offset: 0, limit: 50, query: "", sort: "time", total: 0, rows: [] },
    accLoading: false,
    mail: { email: "", items: [], open: "" },
    combo: { options: [], index: -1, open: false },
    startedAt: Date.now(),
    points: 0,
    runState: "idle"
  };

  /* ============================ 工具 ============================ */
  function esc(s) {
    return String(s === undefined || s === null ? "" : s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  }
  function fmtInt(n) { return Number(n || 0).toLocaleString("en-US"); }
  function fmtDur(ms) {
    if (!ms || ms < 0) return "—";
    const s = Math.floor(ms / 1000);
    if (s < 60) return s + "s";
    const m = Math.floor(s / 60);
    if (m < 60) return m + "m" + String(s % 60).padStart(2, "0") + "s";
    return Math.floor(m / 60) + "h" + String(m % 60).padStart(2, "0") + "m";
  }
  function shortTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    if (isNaN(d.getTime())) return String(iso).slice(11, 19);
    return d.toTimeString().slice(0, 8);
  }
  function trimCode(code) {
    const s = String(code || "");
    if (s.length <= 16) return s;
    return s.slice(0, 7) + "…" + s.slice(-5);
  }
  function debounce(fn, ms) {
    let t = null;
    return function () {
      const args = arguments, self = this;
      clearTimeout(t);
      t = setTimeout(() => fn.apply(self, args), ms);
    };
  }
  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).catch(() => {});
      return;
    }
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); } catch (e) { /* ignore */ }
    document.body.removeChild(ta);
  }

  /* ============================ toast / banner ============================ */
  function toast(msg, kind) {
    const el = document.createElement("div");
    el.className = "toast " + (kind || "info");
    const glyph = kind === "ok" ? "✓" : kind === "err" ? "!" : "i";
    el.innerHTML = '<span class="ti">' + glyph + "</span><span>" + esc(msg) + "</span>";
    $("toasts").appendChild(el);
    setTimeout(() => {
      el.classList.add("out");
      setTimeout(() => el.remove(), 260);
    }, 3600);
  }
  function showBanner(text) { $("bannerText").textContent = text; $("banner").classList.add("show"); }
  function hideBanner() { $("banner").classList.remove("show"); }

  /* ============================ 配置读写 ============================ */
  function readForm() {
    const cfg = {};
    for (const f of FIELDS) {
      const el = $(f);
      if (!el) continue;
      if (el.type === "checkbox") cfg[f] = el.checked;
      else cfg[f] = el.type === "number" ? Number(el.value || 0) : el.value;
    }
    return cfg;
  }
  function writeForm(cfg) {
    if (!cfg || typeof cfg !== "object") return;
    for (const f of FIELDS) {
      const el = $(f);
      if (!el || cfg[f] === undefined || cfg[f] === null) continue;
      if (el.type === "checkbox") el.checked = Boolean(cfg[f]);
      else el.value = cfg[f];
    }
    toggleMailMode();
    refreshBadges();
  }
  function toggleMailMode() {
    const hotmail = $("mailMode").value === "hotmail";
    $("hotmailBox").hidden = !hotmail;
    $("imapBox").hidden = hotmail;
  }
  function markDirty(dirty) {
    state.dirty = dirty;
    $("dirtyHint").textContent = dirty ? "有未保存的修改" : "配置已保存";
    $("dirtyHint").style.color = dirty ? "var(--warn)" : "";
  }
  function refreshBadges() {
    const c = readForm();
    $("badgePanel").textContent = c.panel2Base ? "双面板分摊" : "单面板";
    $("badgeTask").textContent = "目标 " + (c.count || 0) + " · " + (c.workers || 1) + " 路";
    $("badgeMail").textContent = c.mailMode === "hotmail" ? "Hotmail Graph" : "域名邮箱 IMAP";
    $("badgeProxy").textContent = c.proxyPoolUse ? "节点池" : (c.proxyUrl ? "固定代理" : "直连");
  }

  /* ============================ 设置抽屉 ============================ */
  function openDrawer() {
    $("drawer").classList.add("open");
    $("drawer").setAttribute("aria-hidden", "false");
    $("scrim").classList.add("show");
    setTimeout(() => $("panelBase").focus(), 220);
  }
  function closeDrawer() {
    $("drawer").classList.remove("open");
    $("drawer").setAttribute("aria-hidden", "true");
    if (!$("aside").classList.contains("open")) $("scrim").classList.remove("show");
  }

  /* ============================ 运行流 ============================ */
  function pushItem(kind, label, opts) {
    opts = opts || {};
    state.itemSeq += 1;
    const item = {
      id: state.itemSeq,
      kind: kind,
      label: label,
      who: opts.who || "",
      sub: opts.sub || "",
      time: opts.time || new Date().toTimeString().slice(0, 8),
      ts: Date.now(),
      evidence: opts.evidence || "",
      expanded: kind === "failed",
      dur: 0
    };
    state.items.push(item);
    if (state.items.length > 600) state.items.splice(0, state.items.length - 600);
    for (let i = state.items.length - 2; i >= 0; i--) {
      const prev = state.items[i];
      if (prev.kind === "step" && !prev.dur) { prev.dur = item.ts - prev.ts; break; }
    }
    renderStream();
    return item;
  }

  function onStepEvent(e) {
    if (!e || !e.stage) return;
    pushItem("step", e.stage, {
      who: e.email || "",
      time: e.time || undefined,
      sub: e.email ? "线路 " + (e.worker || 1) + " · " + e.email : ""
    });
  }

  function onLogEvent(level, message, time) {
    let lv = String(level || "info").toLowerCase();
    let msg = String(message === undefined || message === null ? "" : message);
    const m = msg.match(/^\[(\d{2}:\d{2}:\d{2})\]\s*\[(\w+)\]\s*(.*)$/);
    if (m) { time = time || m[1]; lv = m[2].toLowerCase(); msg = m[3]; }
    if (/\[state\]/.test(msg)) return;
    if (lv === "error") {
      pushItem("error", "执行失败", { sub: msg, evidence: msg, time: time });
    } else if (lv === "warn") {
      pushItem("warn", "提醒", { sub: msg, evidence: msg, time: time });
    } else if (/(成功|完成|已绑定|已注册|已领取)/.test(msg)) {
      pushItem("success", "完成", { sub: msg, time: time });
    } else {
      pushItem("info", msg, { time: time });
    }
  }

  function itemMatches(item) {
    const f = state.logFilter;
    if (f === "all") return true;
    if (f === "step") return item.kind === "step";
    if (f === "warn") return item.kind === "warn";
    if (f === "error") return item.kind === "error";
    return true;
  }

  function stepIcon(item) {
    if (item.kind === "step") return STAGE_ICON[item.label] || "▸";
    if (item.kind === "error") return "!";
    if (item.kind === "warn") return "▲";
    if (item.kind === "success") return "✓";
    return "·";
  }

  function renderStream() {
    const rail = $("streamRail");
    const visible = state.items.filter(itemMatches);
    if (visible.length === 0) {
      rail.innerHTML =
        '<div class="stream-empty"><span class="glyph">◈</span>' +
        "<div>运行流还没有内容</div>" +
        '<div class="tiny" style="margin-top:6px">开始监督批量后，每一步都会在这里留下可追溯的记录</div></div>';
      return;
    }
    const body = $("streamBody");
    const nearBottom = body.scrollHeight - body.scrollTop - body.clientHeight < 80;
    const html = visible.slice(-400).map((it) => {
      const dur = it.kind === "step" ? (it.dur ? fmtDur(it.dur) : "…") : "";
      const ev = it.expanded && it.evidence
        ? '<div class="step-evidence">' + esc(it.evidence) + "</div>"
        : "";
      return '<div class="step" data-kind="' + it.kind + '" data-id="' + it.id + '">' +
        '<span class="step-icon">' + stepIcon(it) + "</span>" +
        '<div class="step-main">' +
        '<div class="step-label">' + esc(it.label) + (it.who ? ' <span class="who">' + esc(it.who) + "</span>" : "") + "</div>" +
        (it.sub ? '<div class="step-sub' + (it.kind === "error" ? " err" : "") + '">' + esc(it.sub) + "</div>" : "") +
        "</div>" +
        '<div class="step-time">' + esc(it.time) + (dur ? " · " + dur : "") + "</div>" +
        (it.evidence ? '<button class="step-toggle" data-toggle="' + it.id + '" aria-label="展开证据">›</button>' : "") +
        ev +
        "</div>";
    }).join("");
    rail.innerHTML = html;
    if (nearBottom) body.scrollTop = body.scrollHeight;
    $("rcSteps").textContent = fmtInt(state.items.filter((x) => x.kind === "step").length);
  }

  $("streamRail").addEventListener("click", (ev) => {
    const btn = ev.target.closest("[data-toggle]");
    if (!btn) return;
    const item = state.items.find((x) => x.id === Number(btn.dataset.toggle));
    if (!item) return;
    item.expanded = !item.expanded;
    renderStream();
  });

  document.querySelectorAll("[data-log-filter]").forEach((chip) => {
    chip.addEventListener("click", () => {
      document.querySelectorAll("[data-log-filter]").forEach((c) => c.setAttribute("aria-pressed", "false"));
      chip.setAttribute("aria-pressed", "true");
      state.logFilter = chip.dataset.logFilter;
      renderStream();
    });
  });

  /* ============================ 进度 / 收据 ============================ */
  function renderSummary(s) {
    if (!s) return;
    state.summary = s;
    const ok = Number(s.succeeded || 0);
    const target = Number(s.target || 0);
    const fail = Number(s.failedAttempts || 0);
    const pct = target > 0 ? Math.min(100, (ok / target) * 100) : 0;
    const rate = ok + fail > 0 ? ((ok / (ok + fail)) * 100).toFixed(1) + "%" : "—";

    // 用首末成功账号时间差估算速度、平均单号耗时与剩余时间
    let spanMs = 0;
    if (s.firstAt && s.lastAt) {
      const t1 = new Date(s.firstAt).getTime();
      const t2 = new Date(s.lastAt).getTime();
      if (!isNaN(t1) && !isNaN(t2) && t2 > t1) spanMs = t2 - t1;
    }
    const perMin = spanMs > 0 ? (ok / (spanMs / 60000)) : 0;
    const remain = Math.max(0, target - ok);
    $("kpiAvg").textContent = spanMs > 0 && ok > 1 ? fmtDur(Math.round(spanMs / ok)) : "—";
    $("kpiSpeed").textContent = perMin > 0 ? perMin.toFixed(1) + " 个/分钟" : "— 个/分钟";
    $("kpiEta").textContent = perMin > 0 && remain > 0 ? fmtDur(Math.round((remain / perMin) * 60000)) : (remain === 0 ? "已达标" : "—");

    $("kpiOk").textContent = fmtInt(ok);
    $("kpiTarget").textContent = "目标 " + fmtInt(target);
    $("kpiRate").textContent = rate;
    $("kpiFail").textContent = "失败 " + fmtInt(fail);
    $("railMeter").style.width = pct.toFixed(1) + "%";
    $("railStage").textContent = s.currentStage || "待机";
    $("railEmail").textContent = s.currentEmail || "—";
    $("railUpdated").textContent = s.updatedAt ? shortTime(s.updatedAt) : "—";
    $("stageText").textContent = s.currentStage || "待机";
    $("stageMail").textContent = s.currentEmail || "尚未开始任何任务";
    $("stageTag").hidden = !s.running;

    $("rcOk").textContent = fmtInt(ok);
    $("rcFail").textContent = fmtInt(fail);
    $("rcPoints").textContent = fmtInt(s.points || 0);
    $("rcElapsed").textContent = fmtDur(Date.now() - state.startedAt);

    const running = !!s.running;
    $("btnStop").disabled = !running;
    $("btnStopTop").disabled = !running;

    let rs = "idle", text = "待机";
    if (running) { rs = "running"; text = "运行中 · " + ok + "/" + target; }
    else if (target > 0 && ok >= target) { rs = "succeeded"; text = "已达标 " + ok; }
    else if (ok > 0) { rs = "partial"; text = "部分完成 " + ok + "/" + target; }
    else if (fail > 0) { rs = "failed"; text = "已中断"; }
    $("runState").dataset.state = rs;
    $("runStateText").textContent = text;
    $("tabAccCount").textContent = fmtInt(ok);
    if (!state.accLoading && state.acc.offset === 0) scheduleAccRefresh();
  }

  /* ============================ 已绑定账号表 ============================ */
  const scheduleAccRefresh = debounce(() => loadAccounts(), 1500);

  async function loadAccounts() {
    state.accLoading = true;
    const body = $("accBody");
    if (!state.acc.rows.length) {
      body.innerHTML = new Array(8).fill(0).map(() =>
        '<tr><td colspan="4"><div class="skeleton" style="height:11px"></div></td></tr>').join("");
    }
    try {
      const page = await app().GetResultsPage(readForm(), state.acc.offset, state.acc.limit, state.acc.query, state.acc.sort);
      state.acc.rows = page.rows || [];
      state.acc.total = page.total || 0;
      renderAccounts();
    } catch (e) {
      body.innerHTML = '<tr><td colspan="4"><div class="empty"><span class="glyph">!</span><b>读取失败</b></div></td></tr>';
    } finally {
      state.accLoading = false;
    }
  }

  function renderAccounts() {
    const body = $("accBody");
    const rows = state.acc.rows;
    const empty = $("accEmpty");
    empty.hidden = rows.length > 0;
    if (rows.length === 0) {
      body.innerHTML = "";
      empty.querySelector("b").textContent = state.acc.query ? "没有匹配的账号" : "还没有已绑定账号";
      empty.querySelector(".tiny").textContent = state.acc.query
        ? "换个关键字试试，或清空搜索框查看全部"
        : "点击「开始监督批量」开始注册，成功后账号会自动出现在这里";
      $("accRange").textContent = "共 0 条";
      $("pgInfo").textContent = "1/1";
      $("pgFirst").disabled = $("pgPrev").disabled = true;
      $("pgNext").disabled = $("pgLast").disabled = true;
      return;
    }
    body.innerHTML = rows.map((r) =>
      "<tr>" +
      '<td class="mono muted nowrap">' + esc(shortTime(r.createdAt)) + "</td>" +
      '<td class="copy" data-copy="' + esc(r.email) + '" title="' + esc(r.email) + '">' + esc(r.email) + "</td>" +
      '<td class="provider" title="' + esc(r.providerName || "") + '">' +
      (r.signin ? '<span class="sig">✓</span>' : "") + esc(r.providerName || "—") +
      '<button class="cell-btn" data-copy="' + esc(r.providerName || "") + '" title="复制面板账号">⧉</button></td>' +
      '<td class="code" title="设备码 ' + esc(r.signInId || "") + '\nuser_code ' + esc(r.userCode || "") + '">' +
      esc(trimCode(r.signInId || r.userCode)) +
      '<button class="cell-btn" data-copy="' + esc(r.signInId || r.userCode || "") + '" title="复制设备码">⧉</button></td>' +
      "</tr>").join("");
    const total = state.acc.total;
    const from = total === 0 ? 0 : state.acc.offset + 1;
    const to = Math.min(total, state.acc.offset + state.acc.limit);
    $("accRange").textContent = "共 " + fmtInt(total) + " 条 · 显示 " + from + "-" + to;
    const pages = Math.max(1, Math.ceil(total / state.acc.limit));
    const page = Math.floor(state.acc.offset / state.acc.limit) + 1;
    $("pgInfo").textContent = page + "/" + pages;
    $("pgFirst").disabled = state.acc.offset === 0;
    $("pgPrev").disabled = state.acc.offset === 0;
    $("pgNext").disabled = to >= total;
    $("pgLast").disabled = to >= total;
  }

  $("accBody").addEventListener("click", (ev) => {
    const td = ev.target.closest("[data-copy]");
    if (!td) return;
    copyText(td.dataset.copy);
    toast("已复制 " + td.dataset.copy, "ok");
  });

  $("accSearch").addEventListener("input", debounce((e) => {
    state.acc.query = e.target.value.trim();
    state.acc.offset = 0;
    loadAccounts();
  }, 260));

  $("btnAccRefresh").addEventListener("click", () => { state.acc.offset = 0; loadAccounts(); });

  document.querySelectorAll("th.sortable").forEach((th) => {
    th.addEventListener("click", () => {
      const s = th.dataset.sort;
      const asc = s === "time" ? "-time" : s;
      const desc = s === "time" ? "time" : "-" + s;
      if (state.acc.sort === asc) state.acc.sort = desc;
      else if (state.acc.sort === desc) state.acc.sort = asc;
      else state.acc.sort = desc;
      document.querySelectorAll("th.sortable").forEach((x) => x.removeAttribute("aria-sort"));
      const dir = state.acc.sort === desc ? "descending" : "ascending";
      th.setAttribute("aria-sort", dir);
      const arrow = th.querySelector(".arrow");
      if (arrow) arrow.textContent = dir === "descending" ? "↓" : "↑";
      state.acc.offset = 0;
      loadAccounts();
    });
  });

  function gotoPage(delta) {
    const next = state.acc.offset + delta * state.acc.limit;
    if (next < 0 || next >= Math.max(state.acc.limit, state.acc.total)) return;
    state.acc.offset = next;
    loadAccounts();
  }
  $("pgFirst").addEventListener("click", () => { state.acc.offset = 0; loadAccounts(); });
  $("pgPrev").addEventListener("click", () => gotoPage(-1));
  $("pgNext").addEventListener("click", () => gotoPage(1));
  $("pgLast").addEventListener("click", () => {
    state.acc.offset = Math.max(0, (Math.ceil(state.acc.total / state.acc.limit) - 1) * state.acc.limit);
    loadAccounts();
  });

  $("btnExport").addEventListener("click", async () => {
    try {
      const page = await app().GetResultsPage(readForm(), 0, 500, state.acc.query, state.acc.sort);
      const head = ["邮箱", "面板账号", "设备码", "user_code", "签到", "时间"];
      const rows = (page.rows || []).map((r) => [r.email, r.providerName, r.signInId, r.userCode, r.signin, r.createdAt]);
      const csv = [head].concat(rows)
        .map((r) => r.map((c) => '"' + String(c === undefined || c === null ? "" : c).replace(/"/g, '""') + '"').join(","))
        .join("\r\n");
      const blob = new Blob(["\ufeff" + csv], { type: "text/csv;charset=utf-8" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = "minimax-accounts-" + new Date().toISOString().slice(0, 10) + ".csv";
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 2000);
      toast("已导出 " + (page.rows || []).length + " 条", "ok");
    } catch (e) {
      toast("导出失败", "err");
    }
  });

  /* ============================ 收件箱 ============================ */
  const doSearchMailboxes = debounce(async (q) => {
    try {
      const opts = await app().SearchMailboxes(readForm(), q, 40);
      state.combo.options = opts || [];
      state.combo.index = state.combo.options.length ? 0 : -1;
      renderCombo();
    } catch (e) {
      state.combo.options = [];
      renderCombo();
    }
  }, 180);

  function renderCombo() {
    const pop = $("mailComboPop");
    const opts = state.combo.options;
    if (!state.combo.open || opts.length === 0) {
      pop.hidden = true;
      $("mailEmail").setAttribute("aria-expanded", "false");
      return;
    }
    pop.hidden = false;
    $("mailEmail").setAttribute("aria-expanded", "true");
    const groups = { available: [], used: [], reserved: [], skipped: [], other: [] };
    for (const o of opts) {
      const k = groups[o.status] ? o.status : "other";
      groups[k].push(o);
    }
    const labels = { available: "可用", used: "已使用", reserved: "处理中", skipped: "已失效", other: "其他" };
    let html = "";
    let idx = -1;
    for (const key of ["available", "used", "reserved", "skipped", "other"]) {
      if (!groups[key].length) continue;
      html += '<div class="combo-group">' + labels[key] + " · " + groups[key].length + "</div>";
      for (const o of groups[key]) {
        idx += 1;
        html += '<div class="combo-opt" role="option" data-idx="' + idx + '" aria-selected="' + (idx === state.combo.index) + '">' +
          '<span class="st ' + esc(o.status || "") + '"></span>' +
          "<span>" + esc(o.email) + "</span>" +
          (o.note ? '<span class="note" title="' + esc(o.note) + '">' + esc(o.note) + "</span>" : "") +
          "</div>";
      }
    }
    pop.innerHTML = html;
  }

  function pickMailbox(email) {
    state.mail.email = email;
    $("mailEmail").value = email;
    state.combo.open = false;
    renderCombo();
    openInbox(email);
  }

  /* 邮箱快速选择：常用/可用号一键打开收件箱 */
  const QUICK_LIMIT = 8;
  async function loadQuickPicks() {
    const host = $("quickPicks");
    if (!host) return;
    try {
      const opts = await app().SearchMailboxes(readForm(), "", 200);
      const list = (opts || []).filter((o) => o.status === "available" || o.status === "reserved");
      const pick = list.slice(0, QUICK_LIMIT);
      if (!pick.length) {
        host.innerHTML = '<span class="qm">号池暂无可用邮箱，先导入号池</span>';
        return;
      }
      host.innerHTML = pick.map((o) =>
        '<button class="qpill" data-mail="' + esc(o.email) + '" title="' + esc(o.email) + '">' +
        '<span class="st ' + esc(o.status || "") + '"></span>' +
        esc(shortMail(o.email)) + "</button>"
      ).join("");
    } catch (e) {
      host.innerHTML = '<span class="qm">号池未加载</span>';
    }
  }
  function shortMail(email) {
    const s = String(email || "");
    const at = s.lastIndexOf("@");
    if (at < 0) return s.slice(0, 20);
    const local = s.slice(0, at);
    const domain = s.slice(at + 1);
    return (local.length > 16 ? local.slice(0, 15) + "…" : local) + "@" + domain;
  }
  $("quickPicks").addEventListener("click", (ev) => {
    const btn = ev.target.closest("[data-mail]");
    if (!btn) return;
    pickMailbox(btn.dataset.mail);
  });
  $("btnRandomMail").addEventListener("click", async () => {
    try {
      const opts = await app().SearchMailboxes(readForm(), "", 200);
      const avail = (opts || []).filter((o) => o.status === "available");
      if (!avail.length) { toast("号池暂无可用邮箱", "err"); return; }
      const one = avail[Math.floor(Math.random() * avail.length)];
      pickMailbox(one.email);
    } catch (e) {
      toast("随机选择失败", "err");
    }
  });

  $("mailEmail").addEventListener("focus", () => {
    state.combo.open = true;
    doSearchMailboxes($("mailEmail").value.trim());
  });
  $("mailEmail").addEventListener("input", () => {
    state.combo.open = true;
    doSearchMailboxes($("mailEmail").value.trim());
  });
  $("mailEmail").addEventListener("keydown", (ev) => {
    const opts = state.combo.options;
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      if (!state.combo.open) { state.combo.open = true; renderCombo(); return; }
      if (!opts.length) return;
      state.combo.index = ev.key === "ArrowDown"
        ? Math.min(opts.length - 1, state.combo.index + 1)
        : Math.max(0, state.combo.index - 1);
      renderCombo();
      const el = $("mailComboPop").querySelector('[aria-selected="true"]');
      if (el) el.scrollIntoView({ block: "nearest" });
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      if (state.combo.index >= 0 && opts[state.combo.index]) pickMailbox(opts[state.combo.index].email);
      else if ($("mailEmail").value.trim()) pickMailbox($("mailEmail").value.trim());
    } else if (ev.key === "Escape") {
      state.combo.open = false;
      renderCombo();
    }
  });
  $("mailComboPop").addEventListener("mousedown", (ev) => {
    const opt = ev.target.closest(".combo-opt");
    if (!opt) return;
    ev.preventDefault();
    const o = state.combo.options[Number(opt.dataset.idx)];
    if (o) pickMailbox(o.email);
  });
  document.addEventListener("click", (ev) => {
    if (!ev.target.closest(".combo")) { state.combo.open = false; renderCombo(); }
  });

  async function openInbox(email) {
    $("mailNote").textContent = "正在打开 " + email + " …";
    $("mailList").innerHTML = "";
    $("mailDetail").innerHTML = "";
    try {
      const view = await app().MailInboxView(readForm(), email);
      state.mail.items = (view && view.items) || [];
      renderMailList(view);
      const n = state.mail.items.length;
      $("mailNote").textContent = n > 0 ? email + " · 共 " + n + " 封（含垃圾箱）" : email + " · 收件箱为空";
    } catch (e) {
      $("mailNote").textContent = "加载失败：" + (e && e.message ? e.message : e);
    }
  }

  function renderMailList(view) {
    const box = $("mailList");
    if (view && view.error) {
      box.innerHTML = '<div class="empty"><span class="glyph">!</span><b>' + esc(view.error) + "</b></div>";
      return;
    }
    if (!state.mail.items.length) {
      box.innerHTML = '<div class="empty"><span class="glyph">✉</span><b>没有邮件</b><span class="tiny">收件箱与垃圾箱都是空的</span></div>';
      return;
    }
    box.innerHTML = state.mail.items.map((it) =>
      '<div class="mail-row" data-msg="' + esc(it.id) + '">' +
      '<div style="min-width:0"><div class="subj">' + esc(it.subject || "(无主题)") + "</div>" +
      '<div class="meta">' + esc(it.from) + " · " + (it.folder === "junkemail" ? "垃圾箱" : "收件箱") + (it.isRead ? "" : " · 未读") + "</div></div>" +
      '<div class="when">' + esc(String(it.receivedAt || "").slice(5, 16)) + "</div></div>").join("");
  }

  $("mailList").addEventListener("click", async (ev) => {
    const row = ev.target.closest("[data-msg]");
    if (!row) return;
    const id = row.dataset.msg;
    const item = state.mail.items.find((x) => x.id === id);
    if (!item) return;
    state.mail.open = id;
    $("mailDetail").innerHTML = '<div class="mail-body hint">正在加载正文 ' + esc(id) + " …</div>";
    try {
      const msg = await app().MailMessageView(readForm(), state.mail.email, id);
      let html = msg.bodyHtml || "";
      if (!html && msg.bodyText) html = "<pre style='white-space:pre-wrap;'>" + esc(msg.bodyText) + "</pre>";
      $("mailDetail").innerHTML =
        '<div class="mail-body">' +
        '<div class="mb-subject">' + esc(msg.subject || "(无主题)") + "</div>" +
        '<div class="mb-head">发件人 ' + esc(msg.from) + " ｜ 收件人 " + esc(msg.to) + " ｜ " + esc(String(msg.receivedAt || "").slice(5, 16)) + "</div>" +
        "<div>" + html + "</div></div>";
    } catch (e) {
      $("mailDetail").innerHTML = '<div class="mail-body"><span style="color:#ffaaa4">加载正文失败：' + esc(e && e.message ? e.message : e) + "</span></div>";
    }
  });

  /* ============================ 命令面板 ============================ */
  const ACTIONS = [
    { id: "sup-start", title: "开始监督批量", sub: "异常自动重启续跑，直到达标" },
    { id: "sup-stop", title: "停止监督", sub: "结束当前批量循环" },
    { id: "stop-batch", title: "停止当前任务", sub: "中断正在跑的批量" },
    { id: "import-pool", title: "导入号池", sub: "Hotmail Graph 文本 / 文件" },
    { id: "preflight", title: "多线程测活", sub: "校验号池令牌是否可用" },
    { id: "remove-skipped", title: "一键移除失效", sub: "清理令牌过期账号" },
    { id: "checkin", title: "批量补签", sub: "为已注册账号领取每日积分" },
    { id: "mailscan", title: "批量收信", sub: "扫描邮箱 / 识别 Cursor 注册" },
    { id: "fetch-sub", title: "拉取订阅", sub: "更新代理节点池" },
    { id: "check-pool", title: "节点测活", sub: "筛选可用出口 IP" },
    { id: "test-panel", title: "测试面板连接", sub: "校验面板地址与 Key" },
    { id: "save", title: "保存配置", sub: "写入本地配置文件" },
    { id: "settings", title: "打开设置", sub: "面板 / 任务 / 收件 / 代理" },
    { id: "export", title: "导出账号 CSV", sub: "导出当前筛选结果" },
    { id: "inbox", title: "打开收件箱", sub: "按邮箱查看最新邮件" }
  ];

  const SETTINGS = [
    { id: "panelBase", title: "面板地址", sec: "panel" },
    { id: "panelKey", title: "面板密码 / Key", sec: "panel" },
    { id: "panel2Base", title: "面板 2 地址", sec: "panel" },
    { id: "count", title: "目标成功数量", sec: "task" },
    { id: "workers", title: "并行线路数", sec: "task" },
    { id: "mailMode", title: "接码模式", sec: "task" },
    { id: "signinAfterRegister", title: "注册后自动签到", sec: "task" },
    { id: "hotmailPool", title: "号池文件路径", sec: "mail" },
    { id: "proxyUrl", title: "代理地址", sec: "proxy" },
    { id: "proxyPoolUse", title: "使用节点池", sec: "proxy" },
    { id: "proxySubscriptionUrl", title: "订阅地址", sec: "proxy" },
    { id: "fixedPassword", title: "固定密码", sec: "adv" },
    { id: "outputDir", title: "输出目录", sec: "adv" }
  ];

  const palette = {
    open: false,
    index: 0,
    items: []
  };

  function fuzzyMatch(needle, haystack) {
    if (!needle) return true;
    const n = needle.toLowerCase(), h = haystack.toLowerCase();
    if (h.indexOf(n) >= 0) return true;
    let i = 0;
    for (const ch of h) { if (ch === n[i]) i++; if (i === n.length) return true; }
    return false;
  }

  function buildPalette(query) {
    const items = [];
    for (const a of ACTIONS) {
      if (fuzzyMatch(query, a.title + " " + a.sub)) items.push({ group: "操作", title: a.title, sub: a.sub, run: () => runAction(a.id) });
    }
    for (const s of SETTINGS) {
      if (fuzzyMatch(query, s.title)) items.push({ group: "设置", title: s.title, sub: "跳转到设置项", run: () => gotoSetting(s) });
    }
    const q = query.trim().toLowerCase();
    if (q) {
      for (const m of state.combo.options) {
        if (fuzzyMatch(q, m.email)) items.push({ group: "邮箱", title: m.email, sub: m.status || "", tail: "收件箱", run: () => pickMailbox(m.email) });
      }
    }
    return items;
  }

  function renderPalette() {
    const list = $("paletteList");
    if (!palette.items.length) {
      list.innerHTML = '<div class="empty"><span class="glyph">⌕</span><b>没有匹配项</b><span class="tiny">试试「号池」「签到」「面板」</span></div>';
      return;
    }
    let html = "";
    let group = "";
    palette.items.forEach((it, i) => {
      if (it.group !== group) { group = it.group; html += '<div class="palette-group">' + group + "</div>"; }
      html += '<div class="palette-item" role="option" aria-selected="' + (i === palette.index) + '" data-i="' + i + '">' +
        '<div class="pi-main"><b>' + esc(it.title) + "</b><span>" + esc(it.sub || "") + "</span></div>" +
        (it.tail ? '<span class="pi-tail">' + esc(it.tail) + "</span>" : "") +
        "</div>";
    });
    list.innerHTML = html;
    const sel = list.querySelector('[aria-selected="true"]');
    if (sel) sel.scrollIntoView({ block: "nearest" });
  }

  function openPalette() {
    palette.open = true;
    palette.index = 0;
    $("palette").classList.add("open");
    $("paletteInput").value = "";
    palette.items = buildPalette("");
    renderPalette();
    setTimeout(() => $("paletteInput").focus(), 30);
  }
  function closePalette() {
    palette.open = false;
    $("palette").classList.remove("open");
  }

  $("paletteInput").addEventListener("input", (e) => {
    palette.items = buildPalette(e.target.value);
    palette.index = 0;
    renderPalette();
  });
  $("paletteInput").addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { e.preventDefault(); palette.index = Math.min(palette.items.length - 1, palette.index + 1); renderPalette(); }
    else if (e.key === "ArrowUp") { e.preventDefault(); palette.index = Math.max(0, palette.index - 1); renderPalette(); }
    else if (e.key === "Enter") {
      e.preventDefault();
      const it = palette.items[palette.index];
      if (it) { closePalette(); it.run(); }
    } else if (e.key === "Escape") { closePalette(); }
  });
  $("paletteList").addEventListener("mousedown", (e) => {
    const el = e.target.closest(".palette-item");
    if (!el) return;
    e.preventDefault();
    const it = palette.items[Number(el.dataset.i)];
    closePalette();
    if (it) it.run();
  });

  function gotoSetting(s) {
    openDrawer();
    const sec = document.querySelector('.section[data-sec="' + s.sec + '"]');
    if (sec) sec.open = true;
    const el = $(s.id);
    if (el) {
      setTimeout(() => {
        el.scrollIntoView({ block: "center", behavior: "smooth" });
        el.focus({ preventScroll: true });
      }, 260);
    }
  }

  /* ============================ 动作 ============================ */
  async function runAction(id) {
    const cfg = readForm();
    try {
      if (id === "sup-start") {
        await app().SaveConfig(cfg);
        await app().StartSupervised(cfg);
        state.startedAt = Date.now();
        toast("监督批量已启动", "ok");
      } else if (id === "sup-stop") {
        await app().StopSupervised();
        toast("已停止监督");
      } else if (id === "stop-batch") {
        await app().StopBatch();
        toast("已发送停止信号");
      } else if (id === "cursor-bind") {
        await app().SaveConfig(cfg);
        await app().StartCursorBinding(cfg, cfg.count || 1);
        toast("Cursor 绑定已启动", "ok");
      } else if (id === "cursor-stop") {
        await app().StopCursorBinding();
        toast("已发送停止信号");
      } else if (id === "import-pool") {
        openDrawer();
        const sec = document.querySelector('.section[data-sec="mail"]');
        if (sec) sec.open = true;
        setTimeout(() => $("hotmailRaw").focus(), 260);
      } else if (id === "preflight") {
        toast("开始多线程测活…");
        const st = await app().PreflightHotmail(cfg);
        renderPool(st);
        toast("测活完成：可用 " + st.available, "ok");
      } else if (id === "remove-skipped") {
        const st = await app().RemoveSkippedHotmail(cfg);
        renderPool(st);
        toast("已移除失效账号", "ok");
      } else if (id === "checkin") {
        toast("开始批量补签…");
        const msg = await app().CheckinAll(cfg);
        toast(msg || "补签完成", "ok");
      } else if (id === "mailscan") {
        toast("开始批量收信…");
        const msg = await app().MailScanAll(cfg);
        toast(msg || "收信扫描完成", "ok");
      } else if (id === "fetch-sub") {
        toast("正在拉取订阅…");
        const st = await app().ImportProxySubscription(cfg);
        renderNodes(st);
        toast("订阅拉取完成：" + st.total + " 个节点", "ok");
      } else if (id === "check-pool") {
        toast("正在测活节点…");
        const st = await app().CheckProxyPool(cfg);
        renderNodes(st);
        toast("测活完成：存活 " + st.alive + " / " + st.total, "ok");
      } else if (id === "test-panel") {
        const msg = await app().TestPanel(cfg);
        toast(msg, "ok");
      } else if (id === "save") {
        const saved = await app().SaveConfig(cfg);
        writeForm(saved);
        state.saved = saved;
        markDirty(false);
        toast("配置已保存", "ok");
      } else if (id === "settings") {
        openDrawer();
      } else if (id === "export") {
        $("btnExport").click();
      } else if (id === "inbox") {
        document.querySelector('.tab[data-tab="mail"]').click();
        $("mailEmail").focus();
      }
    } catch (e) {
      toast(String(e && e.message ? e.message : e), "err");
    }
  }

  function renderPool(st) {
    if (!st) return;
    $("poolTotal").textContent = fmtInt(st.total);
    $("poolAvail").textContent = fmtInt(st.available);
    $("poolUsed").textContent = fmtInt(st.used);
    const pct = st.total > 0 ? (st.available / st.total) * 100 : 0;
    $("poolMeter").style.width = pct.toFixed(1) + "%";
  }
  function renderNodes(st) {
    if (!st) return;
    $("nodeTotal").textContent = fmtInt(st.total);
    $("nodeAlive").textContent = fmtInt(st.alive);
    $("nodeDead").textContent = fmtInt(st.dead);
    const pct = st.total > 0 ? (st.alive / st.total) * 100 : 0;
    $("nodeMeter").style.width = pct.toFixed(1) + "%";
  }


  /* ============================ 事件绑定 ============================ */
  $("btnSettings").addEventListener("click", openDrawer);
  $("btnCloseDrawer").addEventListener("click", closeDrawer);
  $("btnPalette").addEventListener("click", openPalette);
  $("scrim").addEventListener("click", () => {
    closeDrawer();
    $("aside").classList.remove("open");
    $("rail").classList.remove("open");
    $("scrim").classList.remove("show");
  });
  $("bannerClose").addEventListener("click", hideBanner);
  $("bannerRetry").addEventListener("click", () => { hideBanner(); loadConfig(); });

  $("btnAside").addEventListener("click", () => {
    const a = $("aside");
    a.classList.toggle("open");
    $("scrim").classList.toggle("show", a.classList.contains("open"));
  });
  $("btnRail").addEventListener("click", () => {
    const r = $("rail");
    r.classList.toggle("open");
    $("scrim").classList.toggle("show", r.classList.contains("open"));
  });

  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      document.querySelectorAll(".tab").forEach((t) => t.setAttribute("aria-selected", "false"));
      tab.setAttribute("aria-selected", "true");
      document.querySelectorAll(".tabpane").forEach((p) => p.classList.remove("active"));
      const pane = $("pane-" + tab.dataset.tab);
      if (pane) pane.classList.add("active");
      if (tab.dataset.tab === "mail") loadQuickPicks();
    });
  });

  document.querySelectorAll("#queue .queue-item").forEach((btn) => {
    btn.addEventListener("click", () => runAction(btn.dataset.cmd));
  });

  $("btnStop").addEventListener("click", async () => {
    $("runState").dataset.state = "stopped";
    $("runStateText").textContent = "正在停止…";
    $("btnStop").disabled = true;
    $("btnStopTop").disabled = true;
    try {
      await app().StopBatch();
      await app().StopSupervised();
      toast("已发送停止信号");
    } catch (e) {
      toast(String(e), "err");
    }
  });
  $("btnStopTop").addEventListener("click", () => $("btnStop").click());

  $("btnSave").addEventListener("click", async () => {
    try {
      const saved = await app().SaveConfig(readForm());
      writeForm(saved);
      state.saved = saved;
      markDirty(false);
      toast("配置已保存", "ok");
    } catch (e) { toast("保存失败：" + e, "err"); }
  });
  $("btnRevert").addEventListener("click", () => {
    if (state.saved) writeForm(state.saved);
    markDirty(false);
    toast("已放弃未保存的修改");
  });
  FIELDS.forEach((f) => {
    const el = $(f);
    if (!el) return;
    el.addEventListener("input", () => { markDirty(true); refreshBadges(); });
    el.addEventListener("change", () => { markDirty(true); refreshBadges(); });
  });

  $("mailMode").addEventListener("change", toggleMailMode);
  $("hotmailRaw").addEventListener("input", debounce(updateRawCount, 200));
  $("hotmailRaw").addEventListener("paste", () => setTimeout(updateRawCount, 0));

  function updateRawCount() {
    const v = $("hotmailRaw").value || "";
    const all = v ? v.replace(/\r\n/g, "\n").split("\n").filter((l) => l.trim()) : [];
    const valid = all.filter((l) => {
      const f = l.split("----");
      return f.length >= 4 && f[0].includes("@") && f[1].trim() && f[2].trim();
    }).length;
    const warn = all.length >= 50 && valid < all.length * 0.9
      ? " ⚠ 有效行偏少，可能剪贴不完整，建议用「选择文件并导入」" : "";
    $("rawCount").textContent = "已粘贴 " + all.length + " 行 / 识别有效 " + valid + " 条 / " + v.length + " 字符" + warn;
  }

  $("btnImport").addEventListener("click", async () => {
    updateRawCount();
    const v = $("hotmailRaw").value || "";
    if (!v.trim()) { toast("号池文本为空，请粘贴内容或选择文件导入", "err"); return; }
    try {
      const st = await app().ImportHotmail(readForm());
      renderPool(st);
      loadQuickPicks();
      toast("导入完成：新增可用 " + st.available, "ok");
    } catch (e) { toast("导入失败：" + e, "err"); }
  });

  $("btnImportFile").addEventListener("click", async () => {
    const f = $("rawFile").files && $("rawFile").files[0];
    if (!f) { toast("请先选择 .txt 号池文件", "err"); return; }
    await importRawFile(f);
  });

  async function importRawFile(f) {
    try {
      toast("正在读取 " + f.name + " …");
      const text = await f.text();
      const st = await app().ImportHotmailText(readForm(), text);
      renderPool(st);
      loadQuickPicks();
      toast("文件导入完成，可用 " + st.available + " 个", "ok");
    } catch (e) { toast("文件导入失败：" + e, "err"); }
  }

  /* 号池文本框支持直接拖入 .txt */
  (function enableDropzone() {
    const zone = $("rawDrop");
    if (!zone) return;
    ["dragenter", "dragover"].forEach((ev) =>
      zone.addEventListener(ev, (e) => { e.preventDefault(); zone.classList.add("dragover"); }));
    ["dragleave", "drop"].forEach((ev) =>
      zone.addEventListener(ev, (e) => { e.preventDefault(); zone.classList.remove("dragover"); }));
    zone.addEventListener("drop", (e) => {
      const f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
      if (!f) return;
      if (!/\.(txt|csv)$/i.test(f.name)) { toast("只支持 .txt / .csv 号池文件", "err"); return; }
      importRawFile(f);
    });
  });

  $("btnPreflight").addEventListener("click", () => runAction("preflight"));
  $("btnRemoveSkipped").addEventListener("click", () => runAction("remove-skipped"));
  $("btnFetchSub").addEventListener("click", () => runAction("fetch-sub"));
  $("btnCheckPool").addEventListener("click", () => runAction("check-pool"));
  $("btnPoolStats").addEventListener("click", async () => {
    try { renderNodes(await app().GetProxyPoolStats(readForm())); } catch (e) { /* ignore */ }
  });
  $("btnTestPanel").addEventListener("click", () => runAction("test-panel"));
  $("btnClear").addEventListener("click", async () => {
    try {
      await app().ClearHistory(readForm());
      state.acc.rows = [];
      state.acc.total = 0;
      renderAccounts();
      toast("历史记录已清理", "ok");
    } catch (e) { toast(String(e), "err"); }
  });

  document.addEventListener("keydown", (e) => {
    const typing = /^(INPUT|TEXTAREA|SELECT)$/.test((e.target.tagName || "").toUpperCase());
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      palette.open ? closePalette() : openPalette();
      return;
    }
    if (e.key === "Escape") {
      if (palette.open) { closePalette(); return; }
      if ($("drawer").classList.contains("open")) { closeDrawer(); return; }
      state.combo.open = false;
      renderCombo();
      return;
    }
    if (typing) return;
    if (e.key === "/") { e.preventDefault(); openPalette(); }
    if (e.key.toLowerCase() === "s" && !e.metaKey && !e.ctrlKey) { e.preventDefault(); runAction("sup-start"); }
    if (e.key.toLowerCase() === "i" && !e.metaKey && !e.ctrlKey) { e.preventDefault(); runAction("import-pool"); }
    if (e.key.toLowerCase() === "m" && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      document.querySelector('.tab[data-tab="mail"]').click();
      $("mailEmail").focus();
    }
  });

  /* ============================ 初始化 ============================ */
  async function loadConfig() {
    hideBanner();
    let cfg = null;
    try {
      cfg = await app().GetConfig();
      if (!cfg || typeof cfg !== "object") throw new Error("配置为空");
      writeForm(cfg);
      state.saved = cfg;
      markDirty(false);
    } catch (e) {
      const msg = e && e.message ? e.message : String(e);
      showBanner("加载配置失败：" + msg + " —— 已切换到默认值，可手动填写后保存。");
      onLogEvent("error", "加载配置失败: " + msg);
    }
    try { renderPool(await app().GetHotmailStats(readForm())); } catch (e) { /* ignore */ }
    try { renderNodes(await app().GetProxyPoolStats(readForm())); } catch (e) { /* ignore */ }
    try {
      const lite = await app().GetProgressLite();
      renderSummary(lite);
    } catch (e) { /* ignore */ }
  }

  async function init() {
    renderStream();
    renderAccounts();
    await loadConfig();
    const c = readForm();
    $("panel1").textContent = (c.panelBase || "未配置").replace(/^https?:\/\//, "").slice(0, 22);
    $("railWorkers").textContent = (c.workers || 1) + " 路";
    loadAccounts();
    loadQuickPicks();

    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn("batch:log", (p) => onLogEvent(p.level, p.message, p.time));
      window.runtime.EventsOn("batch:step", (p) => onStepEvent(p));
      window.runtime.EventsOn("batch:progress", (s) => renderSummary(s));
      window.runtime.EventsOn("hotmail:preflight", (p) => {
        $("poolTotal").textContent = fmtInt(p.total);
        $("poolAvail").textContent = fmtInt(p.available);
      });
    }
    setInterval(() => {
      $("rcElapsed").textContent = fmtDur(Date.now() - state.startedAt);
    }, 1000);
    setInterval(async () => {
      try { renderPool(await app().GetHotmailStats(readForm())); } catch (e) { /* ignore */ }
      try { renderNodes(await app().GetProxyPoolStats(readForm())); } catch (e) { /* ignore */ }
    }, 20000);
  }

  document.addEventListener("DOMContentLoaded", init);

})();
