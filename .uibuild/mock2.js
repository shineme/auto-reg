function mkRows(n, offset) {
  var out = [];
  var prov = ["MiniMax997066","MiniMax997970","MiniMax997132","MiniMax998041","MiniMax998310","MiniMax998774"];
  for (var i = 0; i < n; i++) {
    var k = offset + i;
    out.push({
      email: "drewfasthorse" + (16390 + k) + "@hotmail.com",
      providerName: prov[k % prov.length],
      userCode: "M.C561_BAY.0.U.MsaArtifacts.-Cu086H!tUqurVdXcnnnxTZZ56BiV2qFnosbfAbMxUB",
      signInId: "9e5f94bc-e8a4-4e73-b8be-63364c29d753",
      signin: true,
      createdAt: "2026-11-20T13:" + (10 + (k % 40)) + ":02Z"
    });
  }
  return out;
}
window.go = { main: { App: {
  GetConfig: async () => ({
    panelBase: "https://token.shortxmax.com", panelKey: "cbe562d176daaffdc4fde0859dffce85",
    panel2Base: "http://web.open-traffic.com", panel2Key: "cbe562d176daaffdc4fde0859dffce85",
    mailMode: "hotmail", count: 2000, workers: 8, timezoneId: "Asia/Shanghai",
    signinAfterRegister: true, captchaDirect: false, mailViaProxy: false,
    hotmailPool: "/opt/minimax/hotmail-pool.json", hotmailPreflightWorkers: 8,
    proxyStyle: "711", proxyUrl: "socks5://USER278923-zone-custom-region-JP:b36958:global.rotgb.trustsource.live:10000",
    proxyRegionRotate: "JP,US,GB", proxySessTime: 15, proxySticky: true,
    proxySubscriptionUrl: "https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848",
    proxyPoolWorkers: 16, proxyLocalPorts: "18001-18422", proxyPoolUse: true,
    ticketMode: "cdp", chromePath: "/usr/bin/google-chrome",
    outputDir: "/opt/minimax/artifacts", signatureSalt: "I*7Cf%WZ#S&%1RlZJ&C2",
    yySuffix: "ooui", recaptchaSiteKey: "6Lc0gOUtAAAAAM6ovsGBJbA4K1La5ZAOA9B7X39k",
    fixedPassword: ""
  }),
  GetHotmailStats: async () => ({ total: 4854, available: 3031, used: 1820, skipped: 0, reserved: 3 }),
  GetProxyPoolStats: async () => ({ total: 422, alive: 282, dead: 140, checked: 422 }),
  GetProgressLite: async () => ({ target: 2000, succeeded: 1275, failedAttempts: 1111,
    currentStage: "等待面板绑定完成", currentEmail: "drewfasthorse01639@hotmail.com",
    running: true, updatedAt: "2026-11-20T13:16:20Z", points: 400, firstAt: "2026-11-20T11:04:00Z", lastAt: "2026-11-20T13:16:20Z" }),
  GetResultsPage: async (cfg, off, lim, q, sort) => ({ total: 1275, offset: off, limit: lim, rows: mkRows(lim, off) }),
  SearchMailboxes: async (cfg, q, lim) => {
    var names = ["drewfasthorse01639@hotmail.com","estellsaigal558738@hotmail.com","shirlietallchief100485@hotmail.com",
      "concettakohloff802356@hotmail.com","ariettaseigrist47703@hotmail.com","ashamathie51679@hotmail.com"];
    var st = ["available","used","reserved","skipped","used","available"];
    var out = [];
    for (var i = 0; i < names.length; i++) {
      if (!q || names[i].indexOf(q) >= 0) out.push({ email: names[i], status: st[i], note: st[i] === "available" ? "" : "已注册" });
    }
    return out;
  },
  MailInboxView: async () => ({ items: [
    { id: "m1", subject: "登录 Cursor", from: "no-reply@cursor.sh", to: "stoneydeucher952151@hotmail.com", folder: "inbox", isRead: false, receivedAt: "2026-11-18T13:07:00Z" },
    { id: "m2", subject: "MiniMax Code 验证码", from: "no-reply@minimax.io", to: "x@hotmail.com", folder: "inbox", isRead: true, receivedAt: "2026-11-18T12:41:00Z" },
    { id: "m3", subject: "Microsoft 账户安全代码", from: "account-security-noreply@accountprotection.microsoft.com", to: "x@hotmail.com", folder: "junkemail", isRead: false, receivedAt: "2026-11-18T09:02:00Z" }
  ] }),
  MailMessageView: async () => ({ subject: "登录 Cursor", from: "no-reply@cursor.sh", to: "stoneydeucher952151@hotmail.com",
    receivedAt: "2026-11-18T13:07:00Z",
    bodyText: "点击下面的链接登录 Cursor：https://cursor.sh/login?code=xxxxxx" }),
  TestPanel: async () => "面板连接正常（MiniMax Code 已授权）",
  SaveConfig: async (c) => c,
  ImportHotmail: async () => ({ total: 4854, available: 3031, used: 1820 }),
  ImportHotmailText: async () => ({ total: 4854, available: 3031, used: 1820 }),
  PreflightHotmail: async () => ({ total: 4854, available: 3031, used: 1820 }),
  RemoveSkippedHotmail: async () => ({ total: 4854, available: 3031, used: 1820 }),
  ImportProxySubscription: async () => ({ total: 422, alive: 282, dead: 140 }),
  CheckProxyPool: async () => ({ total: 422, alive: 282, dead: 140 }),
  CheckinAll: async () => "补签完成：1275 个账号",
  MailScanAll: async () => "扫描完成：1000/1000，Cursor 注册 200",
  ClearHistory: async () => null,
  StartSupervised: async () => null,
  StopBatch: async () => null,
  StopSupervised: async () => null
}}};
window.runtime = { EventsOn: (n, cb) => {
  if (n === "batch:log") {
    var logs = [
      ["info", "[06:16:20] [info] [drewfasthorse01639@hotmail.com] 申请设备码"],
      ["info", "[06:16:20] [info] [drewfasthorse01639@hotmail.com] 获取风控票据"],
      ["error", "[06:16:20] [error] 获取票据失败: read tcp 127.0.0.1:58446->127.0.0.1:45039: i/o timeout"],
      ["info", "[06:16:23] [info] 等待面板绑定完成"],
      ["info", "[06:17:04] [info] 注册并绑定成功: drewfasthorse01639@hotmail.com -> MiniMax997066"],
      ["info", "[06:17:07] [info] 每日签到领取积分"]
    ];
    logs.forEach(function (l) { cb({ level: l[0], message: l[1], time: "06:16:20" }); });
  }
  if (n === "batch:step") {
    var steps = ["申请设备码","获取风控票据","注册设备","检查账号状态","发送邮箱验证码","轮询邮箱验证码","创建密码并登录","设备授权审批","等待面板绑定完成","每日签到领取积分"];
    steps.forEach(function (s, i) { cb({ stage: s, email: "drewfasthorse01639@hotmail.com", worker: (i % 8) + 1, time: "06:16:20" }); });
  }
} };
setTimeout(function () {
  var h = (location.hash || "").replace("#", "");
  if (h === "mail") document.querySelector('.tab[data-tab="mail"]').click();
  if (h === "drawer") document.getElementById("btnSettings").click();
  if (h === "palette") document.getElementById("btnPalette").click();
  if (h === "combo") {
    var el = document.getElementById("mailEmail");
    el.focus();
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }
}, 900);

setTimeout(function () {
  var p = document.querySelector(".palette");
  var cs = getComputedStyle(p);
  var d = document.createElement("div");
  d.id = "dbg";
  d.textContent = JSON.stringify({
    open: p.className,
    bg: cs.backgroundColor,
    opacity: cs.opacity,
    z: cs.zIndex
  });
  document.body.appendChild(d);
}, 1700);



setTimeout(function () {
  var row = document.querySelector(".quick-row");
  var sc = document.getElementById("quickPicks");
  var btn = document.getElementById("btnRandomMail");
  var d = document.createElement("div");
  d.id = "dbg2";
  d.textContent = JSON.stringify({
    rowW: row ? Math.round(row.getBoundingClientRect().width) : -1,
    rowRight: row ? Math.round(row.getBoundingClientRect().right) : -1,
    scW: sc ? Math.round(sc.getBoundingClientRect().width) : -1,
    scRight: sc ? Math.round(sc.getBoundingClientRect().right) : -1,
    btnW: btn ? Math.round(btn.getBoundingClientRect().width) : -1,
    btnRight: btn ? Math.round(btn.getBoundingClientRect().right) : -1,
    btnDisplay: btn ? getComputedStyle(btn).display : ""
  });
  document.body.appendChild(d);
}, 1700);

