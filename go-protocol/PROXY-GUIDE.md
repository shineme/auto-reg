# 代理接入说明（ZooProxy / Cliproxy / 711）

本工具的「代理与节点池」区块对应关系如下，先看这张表再填表：

| GUI 字段 | 作用 | 举例 |
|---|---|---|
| 代理地址 | 代理账号密码与入口主机 | `socks5://用户名:密码@主机:端口` |
| 代理商家类型 | 参数体系（自动识别一般够用） | `auto` / `clipproxy` / `zooproxy` / `711` |
| 代理区域轮换 | 逗号分隔的国家码，按线路依次切换 | `US,JP,GB` |
| Sticky 时长 | 粘性 IP 保持分钟数（写入 `t-` 或 `sessTime-`） | `15` |
| Cliproxy 州省 | `st-` 州省参数 | `Louisiana` |
| 启用 Sticky 会话 | 关掉=每请求换 IP（Rotating） | 默认开 |
| 每次尝试强制换新会话 | 忽略地址里写死的 `sid`，每次重试换新出口 IP | 建议开 |

## ZooProxy（us-eu.zooproxy.com:5000）

地址样例：

```
socks5://5v8rrjwzio51283-region-Rand-sid-nKYCezDN-t-10:jjxqd63c@us-eu.zooproxy.com:5000
```

用户名是一串参数，含义：

| 片段 | 含义 | 说明 |
|---|---|---|
| `5v8rrjwzio51283` | 账号前缀 | 商家分配的子账号，不要改 |
| `-region-Rand` | 出口国家 | `Rand`=每次随机国家；也可以写死 `US`/`JP`/`DE` 等国家码 |
| `-sid-nKYCezDN` | 会话 ID（粘性 IP） | 相同 sid = 相同出口 IP；换 sid = 换 IP |
| `-t-10` | 粘性时长（分钟） | 该 sid 的 IP 最多保持 10 分钟 |
| `jjxqd63c` | 密码 | |

推荐用法：

1. 想要每次注册都换新 IP：地址里不要写死 sid，或在 GUI 勾选「每次尝试强制换新会话」。
   勾选后程序会在每次重试时自动重写 `-sid-xxxxxxxx`，`-t-` 时长保持不变。
2. 想要固定国家：把地址里的 `region-Rand` 改成 `region-US`，或在 GUI 的「代理区域轮换」填 `US`
   （程序会把 `region-xxx` 整体替换掉）。
3. 想要多国轮换：GUI「代理区域轮换」填 `US,JP,GB`，程序会按线路顺序轮换国家。
4. 想要每请求换 IP（Rotating）：取消勾选「启用 Sticky 会话」，此时不带任何 sid/t- 参数。

ZooProxy 官方能力（动态住宅）：支持 SOCKS5 / HTTP / HTTPS 入口，可按国家 / 州 / 城市定位，
粘性会话最长 120 分钟，也支持按请求轮换。官方说明：https://zooproxy.com/zh/traffic-subject/
本工具用到的三个参数与官方一致：`region-`（国家）、`sid-`（会话/粘性）、`t-`（粘性分钟数，1-120）。

注意：sid 写死 + 长 t- 值 = 长时间用同一个 IP。MiniMax/Cursor 这类风控会很快记住这个出口 IP，
表现就是「票据拿到了但被判 1200063 人机验证未通过」或「验证码邮件发不出来」。
遇到这种情况优先打开「每次尝试强制换新会话」，并把 t- 调小（5-15 分钟）。

## Cliproxy（us.cliproxy.io:3010）

```
socks5://y7ro1299103-region-US-sid-tmVWxDKi-t-5:2e6gbeuz@us.cliproxy.io:3010
```

参数体系与 ZooProxy 相同（region- / sid- / t-），另外支持 `st-<州省>` 指定州省。
程序会自动识别 cliproxy.io 域名，用 Cliproxy 参数体系拼装。

## 711（global.rotgb.trustsource.live:10000）

```
socks5://USER278923-zone-custom:b36958:global.rotgb.trustsource.live:10000
```

711 用的是另一套参数：session-<随机>（粘性会话）+ sessTime-<分钟>（时长）+ region-XX（国家）。
程序会自动识别 trustsource.live / 711proxy.com 域名。

## 节点池模式（订阅节点）

- 「订阅地址」支持多行，每行一个订阅；点「拉取订阅」会并发拉取并合并去重（按 协议|服务器|端口|用户名）。
- 订阅里如果是 vless:// / trojan:// 等非 HTTP/SOCKS 协议，程序会解析出来交给 sing-box，
  每个节点占用一个本地端口（VPS 上由 refresh-nodes.py 生成），再作为「本地代理端口池」使用。
- 「节点测活」做两次校验：出口 IP 回显 + Google reCAPTCHA 域。过不了 Google 的节点会被判死，
  避免取票据时卡 25 秒超时。

## 排查顺序（取码失败 / 1200063 时）

1. 面板能否打开：面板地址建议加入「直连域名」列表（ProxyDirectHosts）。
2. 代理能否打开 Google reCAPTCHA：curl -x <代理> https://www.google.com/recaptcha/api2/anchor
3. 出口 IP 是否已被风控：打开「每次尝试强制换新会话」，或换国家 / 换商家。
4. 收件箱能不能收到邮件：Hotmail 用 Graph 接口查；域名邮箱走 IMAP，
   注意 imap.yeah.net 只有 IPv6 记录，海外 VPS 通常需要加 hosts 指到 IPv4。
