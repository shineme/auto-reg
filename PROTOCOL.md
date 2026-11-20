# MiniMax Code (Global) 邮箱注册与设备码授权协议

> 公开源码说明：本文已用占位符替换真实面板、邮箱、代理、账号与协议常量。`MINIMAX_SIGNATURE_SALT`、`MINIMAX_YY_SUFFIX`、`MINIMAX_RECAPTCHA_SITE_KEY` 等站点参数只从本地环境读取，不随仓库发布。项目不实现 reCAPTCHA/风控绕过、票据伪造、指纹伪装或用于规避的代理轮换；票据只由加载供应商页面的真实浏览器会话生成。`1200063` 只表示当前一次发码请求被拒绝，不等同于整个邮箱域名被封；程序记录失败并按间隔换新账号继续，优先使用官方/授权 API 或供应商测试环境。

## 结论与最终成功记录

不是主要指纹环境的问题。指纹与出口 IP 一致性只影响 reCAPTCHA Enterprise / Akamai 风险评分；实际硬失败来自旧设备码过期或 scoped login context 失效、错误的 `login_redirect`、IAB 与 Node 会话隔离、以及错误的 CAPTCHA action。

协议方式已经完成一次真实注册与授权：

- 面板登录：`https://panel.example.com/?k=<PANEL_KEY>`。
- 邮箱：`mm<random>@example.com`，IMAP 收件箱：`mailbox@example.com`。
- 设备码：`<DEVICE_CODE>`；`login_redirect` 必须是 `/oauth-authorize?user_code=<DEVICE_CODE>`，不要额外添加 `client_id=mcode-public`。
- MiniMax 用户名：`<PROVIDER_USER>`。
- `artifacts/minimax-flow.json`：`status=approved`；`artifacts/api-signin-verify.json`：`state=done`；`artifacts/api-providers-verify.json` 包含 `<PROVIDER_USER>`。
- 可见面板弹窗已显示 `<PROVIDER_USER>` 账号卡片，且没有 `the sign-in code expired`。

## 面板侧协议

1. 面板认证使用 `?k=<PANEL_KEY>`；后续请求保持同一个 `WebRequestSession` cookie。
   `?k=<PANEL_KEY>` 登录页会先返回 `303` 与 `Set-Cookie: <panel_session_cookie>=<key>; Path=/; ...; HttpOnly; SameSite=Lax`，再重定向到去掉 `k` 的面板地址。协议客户端必须把这一步的 `Set-Cookie` 写入 Cookie Jar，并在手动跟随 `Location` 时携带该 Cookie；否则重定向后的页面返回 `GET /?k=... HTTP 401: ... browser sent no sign-in cookie`。Node `fetch(..., { redirect: "follow" })` 不会自动把 303 上的 `Set-Cookie` 变成后续重定向请求的 Cookie，因此用 `redirect: "manual"`、逐跳保存 `Set-Cookie`，并在 302/303 后改用 GET。
2. MiniMax Code (Global) 是插件式供应商，不能直接 `POST /api/signin`。应调用：

   `POST /api/plugin-signin`，JSON body `{"provider":"minimax-code-global","method":0,"inputs":{},"proxy":""}`。

   返回 `id`、`url`、`code`、`state=waiting`。`url` 是本次活码的授权页，例如 `https://account.minimax.io/oauth-authorize?user_code=<DEVICE_CODE>`。
3. 轮询 `GET /api/signin/<id>`；成功终态为 `state=done`，响应带 `user`。`GET /api/providers` 是账号列表的权威来源；本次 `minimax-code-global.account.logins` 已包含 `<PROVIDER_USER>`。
4. 设备码只能用于自己的活码登录上下文；旧码继续登录会产生 `invalid scoped login_redirect` 或 `invalid or expired user_code`。每次运行都从新的 `/api/plugin-signin` 开始，不复用历史窗口或历史码。

## MiniMax 侧协议

1. `POST /oauth2/login` 是账号检查、邮箱验证码登录与新用户创建密码登录的共同入口。页面初始化先 `POST /v1/api/user/device/register`，body `{"uuid":"<UUID>"}`，返回 `data.deviceIDStr`。
2. `login_redirect` 使用 `/api/plugin-signin` 返回 URL 的 `PathAndQuery` 原值，即 `/oauth-authorize?user_code=<DEVICE_CODE>`。成功响应把用户详情写入 `localStorage.user_detail_agent`，协议客户端同步保存到 `artifacts/live-session.json`。
3. 账号检查：`POST /oauth2/login`，body 含 `loginType:"20"`、`email`、`authToken:null`、`deviceID`、`login_redirect`。新邮箱返回 `statusInfo.code=1200056 New user has not set a password`。
4. 发码：`POST /v1/api/user/login/sms/send`，body 为 `{"email":"<EMAIL>","phone":"","randStr":"","ticket":"<TOKEN>","captchaType":"recaptcha"}`。票据必须来自同一真实浏览器会话、同一出口 IP，并由 `grecaptcha.enterprise.execute(siteKey,{action:"send_code"})` 生成。`create_password_continue` action 会返回 `1200063 Verification failed, please try again.`。
5. 新用户创建密码并登录：`POST /oauth2/login`，body 为 `{"loginType":"20","email":"<EMAIL>","authToken":"<RSA 加密密码 base64>","code":"<6位邮箱码>","deviceID":"<deviceIDStr>","login_redirect":"/oauth-authorize?user_code=<DEVICE_CODE>"}`。密码用登录页 RSA PKCS#1 v1.5 公钥加密；响应 `statusInfo.code=0` 表示登录成功。
6. 已有密码账号的邮箱验证码登录可用 `loginType:"21"`、`code`、`deviceID`、`login_redirect`，但本次成功运行走的是新用户 `create-password` 分支。
7. IMAP 接码：`imap.example.com:993` TLS，`mailbox@example.com` + 授权码，`SELECT INBOX` → 按 `SINCE/SUBJECT/TO` 搜索 → `FETCH ... BODY.PEEK[...]` 读取 6 位验证码。`BODY.PEEK` 不把邮件标为已读。
8. 登录后 `GET /oauth2/device/authorize?user_code=<DEVICE_CODE>&app_id=3001` 应为 `status=pending`；`POST /oauth2/device/authorize` body 为 `{"user_code":"<DEVICE_CODE>","decision":"approve","group_id":"<唯一默认且可授权的 group_id>"}`；再查状态应为 `status=approved`。

## 签名、Cookie 与浏览器状态

所有 `account.minimax.io` 的登录/发码 API 请求需要：

- query：`device_platform=web`、`biz_id=3`、`app_id=3001`、`version_code=22201`、`unix=<毫秒>`、`uuid`、`device_id`、`timezone_offset`、`sys_language/lang=en`、`os_name`、`browser_name`、`device_memory`、`cpu_core_num`、`browser_language`、`browser_platform`、`screen_width`、`screen_height`、`user_id`、`client=web`。
- headers：`Content-Type: application/json`、`withCredentials: true`、`x-timestamp=<秒>`、`x-signature=MD5(timestamp + <MINIMAX_SIGNATURE_SALT> + JSON body)`、`yy=MD5(encodeURIComponent(path?query) + "_" + (POST ? JSON body : "{}") + MD5(unix.toString()) + <MINIMAX_YY_SUFFIX>)`。
- Akamai 会下发/校验 `_abck`、`bm_sz`、`ak_bmsc` cookie；页面还加载设备指纹脚本、reCAPTCHA Enterprise 与腾讯 TCaptcha。海外发码使用 `captchaType=recaptcha`、`randStr=""`。

IAB 的 Playwright `evaluate` 是只读隔离作用域，看不到 `window.grecaptcha`、`localStorage`、`document.cookie`，不能直接把票据交给 Node。正确桥接方式是 CDP Chrome：

1. 用持久 profile 启动带 `--remote-debugging-port=9333` 的 Chrome，并配置与面板一致的 `--proxy-server=http://127.0.0.1:2080`。
2. 在该 Chrome profile 打开本次 `/api/plugin-signin` 返回的 `url`。
3. `scripts/cdp-captcha.mjs` 通过 CDP `Runtime.evaluate` 调用 `grecaptcha.enterprise.execute(...,{action:"send_code"})`，并用 `Network.getCookies` 导出同一会话的 Akamai cookie、`uuid`、`deviceIDStr` 到 `STATE_PATH`。
4. `scripts/bridge-captcha.mjs` 可手工写入票据，并记录 action；`minimax-protocol-client.mjs` 会拒绝与请求 action 不一致的票据。

## 可复现执行

入口脚本是 `scripts/run-live-flow.ps1`，参数为 `PANEL_KEY`、`IMAP_USER`、`IMAP_PASSWORD`；IMAP 密码使用邮箱授权码，不是邮箱登录密码。脚本完成：

1. 用 `?k=<PANEL_KEY>` 建立面板会话。
2. 启动/复用 CDP Chrome，调用 `POST /api/plugin-signin` 获取新活码，并保存 `artifacts/live-signin.json`、`artifacts/run-config.json`。
3. 设置 `STATE_PATH=artifacts/live-session.json`、`USER_CODE`、`EMAIL=mm<random>@example.com`、`PASSWORD`、`CAPTCHA_ACTION=send_code`、`LOGIN_REDIRECT=<signIn.url PathAndQuery>`、`MINIMAX_PROXY_URL`。
4. 在 CDP Chrome 打开活码 URL，运行 `scripts/cdp-captcha.mjs` 桥接票据和 cookie。
5. 运行 `scripts/minimax-flow.mjs`：依次执行 `page` → `status=401 login_required` → `register-device` → `check-account=1200056` → `send-code` → IMAP 轮询 → `create-password` → `status=pending` → `authorize` → `status=approved`。
6. 轮询 `GET /api/signin/<id>` 到 `state=done`，检查 `GET /api/providers`，最后打开 `?k=<PANEL_KEY>&view=providers&edit=minimax-code-global` 验证账号卡片可见且没有过期报错。

`scripts/minimax-protocol-client.mjs` 统一维护 cookie、`uuid`、`deviceIDStr`、签名与 HTTP/SOCKS5 代理；支持 `page`、`status`、`register-device`、`check-account`、`send-code`、`login`、`create-password`、`authorize`、`deny`。`scripts/minimax-flow.mjs` 把每步响应写入 `artifacts/minimax-flow.json`。

## 批量注册

可以批量跑，但必须串行并隔离每个账号：一个账号对应一个新 `user_code`、新 Chrome profile、新 CDP 端口、新 `STATE_PATH`、新 `RUN_CONFIG_PATH`、新 `FLOW_PATH` 和独立 IMAP 收件地址过滤，不能复用上一次的票据、Cookie 或登录会话。批量脚本已实现这些隔离：

- `scripts/run-minimax-batch-unit.ps1`：单账号单元，完成 `/api/plugin-signin`、CDP 票据桥接、`minimax-flow.mjs`、轮询 `/api/signin/<id>` 到 `done`、检查 `/api/providers`，并写出该账号的 `result.json`。
- `scripts/run-minimax-batch.ps1`：串行调用单账号单元，逐账号分配 `artifacts/batch/<BatchId>/account-NNN/`，每个账号用不同 CDP 端口和 Chrome profile，失败不阻断后续账号，并持续更新 `summary.json`。
- `scripts/run-minimax-batch.env.example.ps1`：环境变量示例；`IMAP_PASSWORD` 使用邮箱的 IMAP 授权码，不是邮箱登录密码。

运行示例：

```powershell
. .\scripts\run-minimax-batch.env.example.ps1
.\scripts\run-minimax-batch.ps1 -Count 3 -DelaySeconds 5
```

每个账号成功后会生成 `artifacts/batch/<BatchId>/account-NNN/result.json`，包含邮箱、随机密码、面板用户名、设备码和 sign-in id；批次汇总在 `artifacts/batch/<BatchId>/summary.json`。建议先用 `-Count 1` 验证当前出口 IP 与 reCAPTCHA 风控，再逐步提高数量；账号之间保留几秒间隔，避免共用票据作用域和设备码时效问题。

## 协议级快速批量

`scripts/minimax-fast-batch.mjs` 是更快的协议优先版本：面板会话、设备码、注册、IMAP 接码、登录、授权、供应商列表校验全部走 HTTP 协议；浏览器只负责产生 reCAPTCHA Enterprise 票据和 Akamai Cookie。快速版逐账号即时生成 `user_code`，为每个账号启动一个独立临时 Chrome profile、分配一个空闲 CDP 端口，并创建独立 BrowserContext 预热及提交协议请求。账号结束或发生异常后，`finally` 会关闭 BrowserContext、调用 `Browser.close`、用进程树兜底结束 Chrome，并删除该 profile；`session.json.browser` 记录本次 profile、端口和 PID。面板会清理较早的 sign-in 记录，因此不要先批量预生成多个活码再排队等待。

关键边界：

- 不能纯 Node 伪造票据。MiniMax 会校验 `send_code` action、同一浏览器会话、同一出口 IP、Akamai Cookie 和页面上下文；伪造或跨账号复用票据会返回 `1200063`。
- 预热页面用 `https://account.minimax.io/unified-login?login_redirect=<encoded /oauth-authorize?user_code=CODE>`，而不是已过期或已授权的 `/oauth-authorize` 页面；后两类页面不会稳定加载 `enterprise.js`。
- 每个账号仍有自己的活码、票据、Cookie、`uuid`、`deviceIDStr`、邮箱、`STATE_PATH`、临时 Chrome profile 和 CDP 端口；只共享出口 IP。`IMAP_MAILBOX` 必须是 IMAP 文件夹名（通常 `INBOX`），不是收件邮箱地址。
- 协议提交保持串行，避免设备码 scoped login context 和 magpie sign-in 轮询互相踩踏；每次只创建一个浏览器 session。

运行前设置 `PANEL_KEY`、`IMAP_USER`、`IMAP_PASSWORD`；`IMAP_PASSWORD` 使用邮箱 IMAP 授权码。示例：

```powershell
. .\scripts\run-minimax-batch.env.example.ps1
$env:COUNT = "3"
node .\scripts\minimax-fast-batch.mjs
```

票据桥接 dry-run 不创建账号：

```powershell
$env:DRY_RUN = "1"
$env:DRY_RUN_USER_CODE = "TEST-CODE"
node .\scripts\minimax-fast-batch.mjs
```

输出目录是 `artifacts/fast-batch/<BatchId>/`：每个账号有 `session.json`、`run-config.json`、`flow.json`、`signin-verify.json`、`providers.json`、`result.json`；批次汇总在 `summary.json`。首次投产建议 `COUNT=1`，确认 `flow.status=approved`、`signin.state=done`、供应商列表出现新用户后再增大 `COUNT`。

## 守护、实时进度与断点续跑

`scripts/minimax-supervisor.mjs` 负责长时间批量任务。它扫描 `BATCH_ROOT` 下所有时间戳批次，按邮箱或设备码去重 `ok=true` 的 `result.json`，写入 `resume-ledger.json`，再把这些成功记录导入固定 `resume-200` 批次。`TARGET_SUCCESS_COUNT` 表示最终需要的成功账号数；`minimax-fast-batch.mjs` 启动时读取同一批次已有 `result.json`，从 `成功数` 继续补齐，而不是从 `0` 开始。失败的账号目录保留为失败尝试，不计入成功数，后续重启会创建新的账号目录继续尝试。

批量脚本向 stdout 输出单行 JSON 进度事件：`type=progress` 覆盖创建设备码、reCAPTCHA 预热、协议注册、IMAP 接码、OAuth 授权、面板绑定和账号完成；`type=account-result` 携带单个账号的公开结果字段。`minimax-flow.mjs` 通过 stderr 输出 `type=flow-progress` 阶段心跳，批量脚本转换为标准进度事件。守护进程把最新事件原子写入 `supervisor.progress.json`，把 PID、心跳、成功数、失败数、自动恢复次数和退出码写入 `supervisor.status.json`。

Go/Wails GUI 通过轮询这两个文件更新成功/失败计数、阶段文案、当前账号、恢复次数和守护 PID；守护程序由 Windows Job Object 纳管，关闭窗口会停止守护程序、Node 子进程、Chrome/CDP 和本地代理桥，并把状态标记为 `stopped`、删除 `supervisor.lock`。`supervisor.lock` 保存当前守护 PID，阻止重复启动；陈旧锁会在下一次启动清理。子进程因网络、CDP 或 Node 异常退出时，守护进程等待 5 秒后重启 `minimax-fast-batch.mjs`，复用 `BATCH_ID=resume-200` 和落盘成功记录，直到 `succeeded >= TARGET_SUCCESS_COUNT` 或用户关闭软件/结束守护进程。

发码返回 `1200063 Verification failed, please try again.` 且 `debugInfo` 为“人机验证未通过”时，请求在邮件发送前已被 MiniMax 拒绝；IMAP 主机、授权码和搜索条件正常也不会有新验证码。这个响应只证明当前票据、设备码、会话和邮箱组合的本次尝试被拒绝，不能据此推断整个邮箱域名被风控。批量脚本把它作为普通失败写入 `result.json`，守护进程在间隔后以全新邮箱、设备码、票据会话和临时 Chrome profile 自动继续，直到达到目标成功数；不复用旧设备码，也不实现风控规避。守护进程启动时删除 `risk-state.json`、各批次旧 `risk-control.json` 以及历史停机风控状态，GUI 也提供「清理运行状态」与「清空历史记录」两个本机操作。前者只清理旧风险与守护状态并保留成功账号，后者额外删除本机输出目录里的历史账号目录、汇总和台账；两者都不会删除已经绑定到面板的账号。只有 reCAPTCHA 页面或本地代理环境持续不可用时才进入 `environment-unavailable`；面板或协议请求的 `ECONNRESET`、连接意外关闭属于瞬时网络错误，守护进程只结束当前子进程并按间隔自动恢复，不把它误判成票据环境故障。`MINIMAX_BATCH_SCRIPT` 仅供离线测试注入假子脚本。

代理模式需要先确认 SOCKS5 认证、本地 HTTP CONNECT 桥、`account.minimax.io` 与 Google reCAPTCHA 资源都能从同一出口访问。仅端口/认证成功不代表票据页可用；若代理出口到 `account.minimax.io` 超时，会出现 `reCAPTCHA Enterprise did not become ready`，此时换 IP 只能解决出口可达性问题。发码阶段的 `1200063` 只按当次会话失败处理，逐个账号重新生成票据、设备码和临时 profile，不据此推断域名被封，也不实现风控规避。每个账号启动独立临时 Chrome，切换代理或直连后不会出现旧 CDP Chrome 仍指向已关闭本地桥端口的问题。

紧凑 SOCKS5 代理格式由 Go 代理桥转换成标准 URL，再以本地 HTTP CONNECT 代理供 Node 和 Chrome 使用。2026-11-14 的 200 账号正式批次使用直连：早先紧凑 SOCKS5 出口下前三个账号均因 `reCAPTCHA Enterprise did not become ready` 失败，而直连票据 dry-run 能取得有效票据、Akamai Cookie、`uuid` 和 `deviceId`。

面板连接可以独立做非 mutating 检查：设置 `PANEL_KEY`、`PANEL_BASE` 和 `PANEL_CHECK_ONLY=1`，脚本只访问登录页并读取 `/api/providers`，不调用 `/api/plugin-signin`、不创建设备码、不注册账号。GUI 的「测试面板连接」按钮使用同一路径；成功输出包含 `ok=true` 与 `cookies=["<panel_session_cookie>"]`。

```powershell
$env:PANEL_CHECK_ONLY = "1"
$env:PANEL_BASE = "https://panel.example.com"
$env:PANEL_KEY = "<PANEL_KEY>"
node .\scripts\minimax-fast-batch.mjs
```

## 验收证据

- `artifacts/minimax-flow.json`：完整步骤与 `status=approved`。
- `artifacts/live-signin.json`：本次 `/api/plugin-signin` 的 `id`、`url`、`code`。
- `artifacts/api-signin-verify.json`：`state=done`、`user=<PROVIDER_USER>`。
- `artifacts/api-providers-verify.json`：`/api/providers` 含 `<PROVIDER_USER>`。
- `artifacts/live-session.json`：成功登录后的 cookie、`deviceIDStr`、`user_detail_agent` 与 `captcha.action=send_code`。
- `artifacts/provider-modal.jpg`：重新加载后的面板弹窗截图，可见 `<PROVIDER_USER>`。
- 面板可见弹窗：`MiniMax Code (Global)` → 账号列表 → `<PROVIDER_USER>`，无 `the sign-in code expired`。

## 设备授权页状态机

1. `GET /oauth2/device/authorize?user_code=<DEVICE_CODE>&app_id=3001`，`credentials: include, cache: no-store`：
   - 未登录：HTTP 401，`{"error":"login_required","login_url":"/unified-login?login_redirect=..."}`。
   - 设备码过期：HTTP 400，`{"error":"invalid_request","error_description":"invalid or expired user_code"}`。
   - 已登录且待确认：`status=pending`，响应含 `client_info`、`account_info`、`scopes`、`groups`。
   - 已授权：`status=approved`；已拒绝：`status=denied`。
2. 确认授权：`POST /oauth2/device/authorize`，`credentials: include, cache: no-store`，body 为 `{"user_code":"<DEVICE_CODE>","decision":"approve","group_id":"<唯一 is_default 且 can_authorize 的 group_id>"}`。拒绝时 body 为 `{"user_code":"<DEVICE_CODE>","decision":"deny"}`。HTTP 409 且响应含 `status` 表示重复决策；410 或 `authorization_expired` 表示设备码过期。
3. magpie 后端轮询同一设备授权状态；只有 POST 成功变为 `approved` 后，`/api/signin/<id>` 才会从 `waiting` 到 `done`，面板才会新增账号卡片。

## 指纹环境诊断

不是主要根因，但需保持环境一致：

- 从 Node 直连和经 `127.0.0.1:2080` 全局代理调用 `POST /v1/api/user/device/register` 均返回 HTTP 200、`deviceIDStr` 和 Akamai cookie；说明协议签名、设备注册和出口 IP 本身不是根因。
- 设备码的 scoped login context 失效时，`GET /oauth2/device/authorize` 返回 `400 invalid or expired user_code`；对失效码继续 `POST /oauth2/login` 返回 `1000021 Login failed ... invalid scoped login_redirect: ... user_code not found or expired`。这是时效与作用域问题，不是 IMAP、邮箱域名或 RSA 加密错误。
- reCAPTCHA 是独立前置条件：票据必须由同一浏览器会话、同一出口 IP、同一 `send_code` action 产生并桥接。IAB 与 Node 无法共享登录会话和 CAPTCHA 票据是工具链/会话隔离问题，不等同于浏览器指纹被封。

因此优先修活码时效、会话状态、正确 `LOGIN_REDIRECT` 与票据桥接，而不是反复更换指纹。保持干净的普通 Chrome 会话、同一代理出口 IP，并且每个设备码只开一个登录窗口；程序不为规避风控而伪造指纹或轮换代理。

## 排障要点

- 历史设备码不写入仓库；不要复用已失效码。`<DEVICE_CODE>` 表示一次成功记录中的占位符。
- `LOGIN_REDIRECT` 必须是面板 sign-in URL 的 `PathAndQuery`：`/oauth-authorize?user_code=<CODE>`；`client_id=mcode-public` 只存在于旧探测脚本的页面 URL，不用于 `POST /oauth2/login`。
- 发码 action 必须是 `send_code`；`create_password_continue` 票据会返回 `1200063`。
- 本地 CDP HTTP 调用若被系统代理拦成 502，用 Node `fetch` 或 .NET `HttpClientHandler.UseProxy=$false`；`CDP_TARGET_URL` 精确匹配失败时，可在同一 CDP Chrome profile 打开当前 `signIn.url` 后置空该变量。
- `scripts/minimax-bundle-audit.mjs`、`scripts/minimax-page-audit.mjs` 可在前端改版后重新提取 Next.js chunk 的接口路径与状态机；`scripts/imap-probe.mjs` 用于单独检查接码邮箱。
- 面板返回 `401 ... browser sent no sign-in cookie` 时，优先检查登录页 303 的 `Set-Cookie` 是否被保存并用于下一次重定向请求；这不是浏览器指纹或 Cookie 被别的端口服务清掉的首要证据。每主机共享 Cookie，同源不同端口的服务确实能清掉站点数据，但协议客户端自己的 Cookie Jar 问题应先排除。
- 配置了上游代理时，本地代理桥会设置 `NODE_USE_ENV_PROXY=1`、`HTTP(S)_PROXY`、`ALL_PROXY` 和 `MINIMAX_PROXY_URL`，让 Node 面板请求、MiniMax 协议请求与 CDP Chrome 共用同一出口；`NO_PROXY` 保留 `localhost/127.0.0.1/::1`，避免本机 CDP 控制请求也被代理。代理留空时禁用 Node 环境代理，Chrome 也不附加 `--proxy-server`。
- GUI 接受 `socks5://user:pass@host:port`、`http://user:pass@host:port`、缺少 `@` 的 `scheme://user:pass:host:port`，以及 Markdown 转义后的 `http\://...`；规范化后统一转成本地 HTTP CONNECT 桥。
- 遇到 `1200063` 时只记录本次尝试失败；检查票据 action、设备码时效、会话状态和出口一致性后按间隔换新账号继续，不要把它误判为域名风控。不要伪造票据、复用跨账号票据或规避风控。票据环境不可用时也立即停止并检查出口或代理；恢复失败时联系供应商或使用授权 API/测试环境。


## Cursor 授权流程（面板 provider = cursor）

面板 `POST /api/plugin-signin {"provider":"cursor","method":0,"inputs":{},"proxy":""}` 返回
`{"id":"<SIGNIN_ID>","url":"https://cursor.com/loginDeepControl?challenge=..&uuid=..&mode=login&redirectTarget=cli","state":"waiting"}`，
面板随后轮询 `GET /api/signin/<SIGNIN_ID>`，浏览器完成登录后该记录变成 `done` 并带上面板账号。

浏览器侧（CDP 驱动真实 Chrome，`go-protocol/cursor*.go`）：

1. 打开 `signIn.url` -> 点 `Continue to sign in` -> 跳 `authenticator.cursor.sh`。
2. 邮箱页填邮箱并提交 -> 密码页切到 `button[value="magic-code"]`（邮箱取码登录）。
3. `POST /me/messages` (Graph) 取标题含 Cursor 的 6 位验证码 -> 写入 `input[name=code]` -> 回车提交。
4. 授权确认页 `Sign in to Cursor CLI` -> 点 `Sign in`（真实鼠标事件，跳过 Cookie 同意框）。
5. Cursor CLI 会再发一封 `Confirm a change to your Cursor account` 邮件，信里的
   `https://cursor.com/confirm-action#t=<TOKEN>` 必须在**同一个浏览器**里打开（新标签页），
   并点页面上的 `Confirm`。
6. 面板 `/api/signin/<SIGNIN_ID>` 变 `done` -> 读 `/api/providers` 里 cursor 的 `account.logins` 校验落库。

要点：

- 取码与取确认链接用不同的邮件过滤：`kind=cursor` 排除 `Confirm a change` 邮件（避免把确认信里的数字当验证码），
  `kind=cursor-confirm` 只认 `Confirm a change` 邮件；确认链接要读 Graph 的原始 HTML 正文（`body.content`），
  清洗后的正文会把 `href` 去掉。
- 人机验证（Cloudflare Turnstile）不自动绕过：程序把 Chrome 窗口摆到屏幕左上角并置前，
  由用户点一次 "I'm human"，之后流程自动继续。
- 每次运行用独立的 Chrome profile 目录，结束即删；面板与协议请求共用同一出口 IP。
- 界面语言可能是中文，人机验证提示要同时认 `确认您是真人 / 验证您是人类 / 人机验证` 等中文文案。
- OneTrust 的 Cookie 同意框和全屏遮罩（`.onetrust-pc-dark-filter`）会挡住按钮的坐标判定，
  取坐标前先把这些遮罩设为 `display:none`，否则真实鼠标点击会落在遮罩上。
- 授权确认页上的 `Sign in` 会被一次性引导清单（`Connect your repos to finish setup` /
  `Build cloud-based code automations`）盖住：先点 `Maybe later` 关掉引导，再点 `Sign in`。
- 浏览器对 `cursor.com` 不走本地代理桥（上游会拒 CONNECT），面板与 Graph 取信仍走上游代理；
  用 `MINIMAX_CURSOR_DIRECT=1` 控制。
- 批量入口：CLI `-cursor-batch -cursor-count N`，GUI「绑定 Cursor 账号」按钮；
  号池用 `outputDir/cursor-test/pool.json` 副本，不污染主号池。

## 号池的平台维度（MiniMax / Cursor 复用同一批邮箱）

号池里每个邮箱带一份 `platforms` 映射，键是平台 ID（`minimax` / `cursor`），
值是该邮箱在这个平台上的状态：

```json
{
  "email": "stoneydeucher952151@hotmail.com",
  "status": "used",
  "platforms": {
    "minimax": { "status": "used", "updatedAt": "2026-11-19T05:03:51Z" },
    "cursor":  { "status": "used", "panelUser": "stoneydeucher952151@hotmail.com",
                 "signInId": "t9AB9rb3-k2r", "registeredAt": "2026-11-20T12:01:19Z" }
  }
}
```

规则：

- `used` / `reserved` 是**平台级**的：同一个邮箱在 MiniMax 上注册过，照样能拿去绑 Cursor，
  反之亦然，互不影响。
- `skipped` 是**邮箱级**的：Graph 令牌失效，所有平台都不能再用（写 `skipped` 时会同步所有平台）。
- 老号池（没有 `platforms` 字段）读取时自动迁移：旧的 `status=used` 记到 `minimax`，
  `cursor` 视为可用，所以历史号能直接拿来做 Cursor 绑定。
- 领号按平台排队：`ClaimFor(platform)` 只看该平台的状态，
  没在该平台用过的邮箱排在最前，优先复用。
- MiniMax 与 Cursor 共用同一个号池文件，用 `.lock` 文件做跨进程互斥，避免互相覆盖。
- GUI 资源区新增「MiniMax 可用 / Cursor 可用 / 两平台通用」三行统计。

接口：

```go
store.ClaimFor(PlatformCursor)                     // 按平台领号
store.FinalizeFor(PlatformCursor, idx, email, ...)  // 按平台回写状态
store.MarkPlatformUsed(PlatformCursor, idx, email, panelUser, signInID)
pool.StatsByPlatform()                              // 各平台分别的统计
```
