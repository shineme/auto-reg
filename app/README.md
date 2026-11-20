# MiniMax 协议注册工作台

Go + Wails v2 桌面程序，用于配置 magpie 面板、域名邮箱 / Hotmail Graph 接码、HTTP/SOCKS5 代理和批量注册参数，并驱动 `scripts/minimax-fast-batch.mjs` 完成协议级注册与绑定。

## 功能

- 配置面板地址、面板密钥、邮箱域名和前缀。
- 配置收件邮箱、IMAP 主机/端口/登录名/授权码。
- 也可切换到 Hotmail Graph 模式：导入 `email----password----clientId----refreshToken`，先测试收信，再用于批量注册。
- 支持 `socks5://user:pass@host:port`、`http://user:pass@host:port`、省略 `@` 的 `socks5://user:pass:host:port` 和转义后的 `http\://...`。
- 自动把 SOCKS5 或带认证的 HTTP 上游代理桥接成本地 HTTP CONNECT 代理，供 Node 协议脚本和 Chrome 共用同一出口。
- 配置批量数量、Chrome 路径、协议脚本目录和输出目录；CDP 端口由每个临时浏览器自动分配。
- 启动/停止运行、票据 dry-run、实时日志、运行状态、最近结果表、打开输出目录。
- 实时显示成功数、失败数、当前账号阶段、自动恢复次数、恢复批次和守护进程 PID。
- `scripts/minimax-supervisor.mjs` 聚合输出目录里所有时间戳批次的成功账号，写入 `resume-ledger.json`，生成固定 `resume-200` 恢复批次；子进程异常退出后自动以同一批次重启，直到达到目标成功数。
- 发码接口返回 `1200063 / 人机验证未通过` 时，只把当前账号记为失败尝试，不再据此判定整个邮箱域名被风控、不再停机。守护进程间隔后以新邮箱、新设备码和独立临时 Chrome profile 自动继续；启动时会清理旧风控标记与守护状态。
- reCAPTCHA 页面或票据环境不可用（例如代理出口无法访问 `account.minimax.io`）时立即进入 `environment-unavailable`、`running=false`，提示检查出口或代理；每个账号使用独立临时 Chrome profile，账号结束后立即关闭进程并删除 profile。
- 面板或协议请求遇到 `ECONNRESET`、连接意外关闭等瞬时网络错误时，只结束当前子进程并按间隔自动恢复，不再误判为 reCAPTCHA 环境不可用或整机停机。
- 代理入口可通过 `MINIMAX_AUTOSTART=1` 启动 GUI 后自动开始批次；这样内置代理桥与守护进程保持同一进程组，避免手动启动 Node 时复用到 GUI 已关闭的本地桥端口。
- 守护进程向下游脚本显式清空自动启动变量，避免环境变量污染导致重复启动。
- 守护进程把进度写入 `supervisor.status.json` 与 `supervisor.progress.json`；重新打开 GUI 可继续观察同一批次的落盘进度。
- 关闭窗口会调用 `OnShutdown`，通过 Windows Job Object 和显式进程树清理停止守护程序、Node 子进程、Chrome/CDP 与本地代理桥，并把 `supervisor.status.json` 标记为 `stopped`、删除 `supervisor.lock`。
- 「测试面板连接」按钮只做非 mutating 检查：登录面板、保存 303 的会话 Cookie、读取 `/api/providers`，不会调用 `/api/plugin-signin` 或创建账号。
- 配置保存在当前用户配置目录，权限为仅当前用户可读写；日志输出会遮蔽凭据和票据。每个账号的 `session.json` 记录该次临时 profile 路径、CDP 端口和 Chrome PID，便于核对隔离与及时清理。
- 「清理运行状态」只删除本机旧风控标记与守护状态并保留成功账号；「清空历史记录」额外删除本机输出目录中的历史账号目录、汇总和台账，不删除已经绑定到面板的账号。

## 运行前提

- Windows，Go 1.26+，Wails CLI v2.17+。
- Node.js 24+。
- Google Chrome。
- 仓库根目录的 `scripts/minimax-fast-batch.mjs`、`scripts/minimax-flow.mjs`、`scripts/minimax-protocol-client.mjs`、`scripts/imap.mjs` 等协议脚本。
- 仓库根目录的 `scripts/hotmail-graph.mjs`，用于 Hotmail / Outlook Graph 导入、账号池和验证码轮询。
- 仓库根目录的 `scripts/minimax-supervisor.mjs`，用于断点续跑和自动恢复。
- 可用的 magpie 面板密钥、域名邮箱 catch-all、IMAP 授权码和出口代理。
- 或者一批可用的 Hotmail / Outlook Graph 账号。

## 开发运行

```powershell
$env:Path += ';' + (go env GOPATH) + '\bin'
cd app
wails dev
```

首次启动前，在本机用户环境变量中配置 `MINIMAX_SIGNATURE_SALT`、`MINIMAX_YY_SUFFIX`、`MINIMAX_RECAPTCHA_SITE_KEY`。这些站点参数不写入仓库。随后填写 GUI 配置，先点「测试面板连接」确认 `?k=` 登录 303 的会话 Cookie 和 `/api/providers` 可用；再点「票据 dry-run」确认真实浏览器页面能取得 reCAPTCHA 票据、Akamai Cookie、`uuid` 和 `deviceIDStr`，最后用 `批量数量=1` 试跑。

「测试面板连接」不会注册账号，适合在修改面板密钥、面板地址或代理后立即排查 `401 ... browser sent no sign-in cookie`。该 401 通常是协议客户端没有把登录页 303 响应中的 `Set-Cookie` 带入重定向后的请求，不是浏览器指纹问题；脚本已改为 `redirect: "manual"` 并维护 Cookie Jar。完整说明见 `PROTOCOL.md` 的「面板侧协议」和「排障要点」。

## 构建

```powershell
$env:Path += ';' + (go env GOPATH) + '\bin'
cd app
wails build -platform windows/amd64
```

产物默认在 `app/build/bin/`。

## 代理格式

```text
socks5://example-user:pass@proxy.example.com:10000
http://example-user:pass@proxy.example.com:10000
socks5://example-user:pass:proxy.example.com:10000
http\://example-user:pass:proxy.example.com:10000
```

留空表示直连。不要在不同账号间复用设备码或票据；程序会为每个账号创建独立 BrowserContext 和独立协议状态文件。程序不包含 reCAPTCHA 绕过、票据伪造、指纹伪装或用于规避的代理轮换；`1200063` 只作为本次尝试失败处理，由守护进程换新账号自动继续。

## 验证口径

成功账号必须同时满足：

- `flow.json` 中 `status=approved`；
- `/api/signin/<id>` 为 `state=done`；
- `/api/providers` 包含 `user`；
- 面板弹窗可见账号卡片且没有 `the sign-in code expired`。

完整协议说明见仓库根目录 `PROTOCOL.md`。

## 真实批量验证

2026-11-14 用同一套 Go runner / Node 协议脚本以 `COUNT=2` 完成真实批量验证：批次 `artifacts/gui-batch/20261114-013703/summary.json` 为 `succeeded=2, failed=0`；两个账号均有 `flowStatus=approved`、`signInState=done`，且最终 `/api/providers` 同时包含 `<PROVIDER_USER_A>` 与 `<PROVIDER_USER_B>`。

验证期间修复了三个实际运行问题：GUI 的邮箱域名/前缀未传入协议脚本；`IMAP_MAILBOX` 错用收件邮箱地址导致 `A2 NO SELECT Folder not exist`；一次性预生成多个设备码会让面板清理较早的 sign-in 记录，改为逐账号即时创建设备码。失败的子进程也会直接返回 stdout/stderr，不再被误报成 `Invalid JSON`。

## 断点续跑与恢复验证

正式批量由守护进程管理。守护进程启动时扫描 `BATCH_ROOT` 下所有时间戳批次以及固定恢复批次中的 `account-*/result.json`，按邮箱、设备码或 sign-in id 去重成功账号，复制到 `resume-200`，并把台账写入 `resume-ledger.json`。后续只补齐 `目标成功数 - 已成功数` 个账号，失败尝试会继续重试且不计入成功数。

每个账号在创建设备码、预热 reCAPTCHA、注册设备、发送验证码、IMAP 接码、创建账号、OAuth 授权和等待面板绑定时会输出结构化进度事件；GUI 同时读取守护状态文件和 `summary.json`，因此即使 GUI 被重新打开，也能显示已有成功数。`supervisor.lock` 保存当前守护进程 PID，避免重复启动造成多个批次互相抢占。守护进程异常退出后，单实例锁会识别陈旧 PID；重新启动 GUI 或再次点击开始即可继续同一恢复批次。

2026-11-14 的 200 账号任务使用直连继续；此前紧凑 SOCKS5 代理批次出现 `reCAPTCHA Enterprise did not become ready`，因此该代理配置不作为这次 200 账号验证的出口。

恢复验证包含 `node scripts/minimax-resume-test.mjs`：它构造一个历史成功账号，模拟守护进程中断，再在同一恢复批次放入第二个成功账号；重新启动后守护进程应识别两个成功账号、写出 `finishedAt`、释放单实例锁，且不会重复创建成功账号。隔离测试还验证了历史成功账号恢复、实时进度文件、强杀子进程后的 `restartCount` 增长和同一目标继续补齐。

`node scripts/minimax-domain-risk-test.mjs` 使用假子脚本验证：发码返回 `1200063` 只记为本轮失败，守护进程会自动恢复再试一次，不进入 `domain-blocked`；写入旧风控标记后重新启动，守护进程会删除旧标记并继续，而不是被历史状态永久拦住。真实运行中 `1200063` 表示 MiniMax 在发送邮件前拒绝了这次请求，IMAP 没有新验证码属于预期结果；程序按间隔换新账号继续，不复用旧设备码。
