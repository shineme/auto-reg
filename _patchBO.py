import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\PROTOCOL.md'
doc = '''

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
'''
io.open(p,'a',encoding='utf-8',newline='').write(doc)
print('ok')
