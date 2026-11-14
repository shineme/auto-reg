param(
  [string]$PanelKey = $env:PANEL_KEY,
  [string]$ImapUser = $env:IMAP_USER,
  [string]$ImapPassword = $env:IMAP_PASSWORD,
  [string]$MailDomain = $env:MAIL_DOMAIN,
  [string]$ProxyUrl = "http://127.0.0.1:2080",
  [string]$ChromeProfile = (Join-Path $PWD "artifacts/chrome-cdp-profile"),
  [int]$CdpPort = 9333
)

$ErrorActionPreference = "Stop"
if (!$PanelKey -or !$ImapUser -or !$ImapPassword) {
  throw "PANEL_KEY, IMAP_USER and IMAP_PASSWORD are required"
}
if (!$MailDomain) { $MailDomain = "example.com" }

$base = if ($env:PANEL_BASE) { $env:PANEL_BASE } else { "https://panel.example.com" }
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-WebRequest -Uri "$base/?k=$PanelKey&view=providers&edit=minimax-code-global" -WebSession $session -UseBasicParsing -TimeoutSec 15 | Out-Null

$chrome = "C:\Program Files\Google\Chrome\Application\chrome.exe"
$running = Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" | Where-Object { $_.CommandLine -like "*remote-debugging-port=$CdpPort*" }
if (!$running) {
  & $chrome "--remote-debugging-port=$CdpPort" "--user-data-dir=$ChromeProfile" "--no-first-run" "--no-default-browser-check" "--disable-background-networking" "--window-position=-32000,-32000" "--window-size=3440,1440" "--proxy-server=$ProxyUrl" "about:blank" | Out-Null
}

$env:MINIMAX_PROXY_URL = $ProxyUrl
$env:STATE_PATH = Join-Path $PWD "artifacts/live-session.json"
$env:USER_CODE = ""
$env:EMAIL = "mm$([guid]::NewGuid().ToString('N').Substring(0,10))@$MailDomain"
$env:PASSWORD = "Mm!$([guid]::NewGuid().ToString('N'))"
$env:CAPTCHA_ACTION = "send_code"
$env:IMAP_USER = $ImapUser
$env:IMAP_PASSWORD = $ImapPassword
$env:FLOW_PATH = Join-Path $PWD "artifacts/minimax-flow.json"
$env:LOGIN_REDIRECT = ""

Remove-Item -LiteralPath $env:STATE_PATH -ErrorAction SilentlyContinue

$body = '{"provider":"minimax-code-global","method":0,"inputs":{},"proxy":""}'
$signIn = Invoke-RestMethod -Uri "$base/api/plugin-signin" -WebSession $session -Method Post -Headers @{ "Content-Type" = "application/json" } -Body $body -TimeoutSec 30
$env:USER_CODE = $signIn.code
$env:LOGIN_REDIRECT = ([uri]$signIn.url).PathAndQuery
$signIn | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $PWD "artifacts/live-signin.json") -Encoding UTF8
$env:CDP_TARGET_URL = $signIn.url

$config = @{ userCode = $signIn.code; email = $env:EMAIL; password = $env:PASSWORD; loginRedirect = $env:LOGIN_REDIRECT; createdAt = (Get-Date).ToUniversalTime().ToString("o") }
$config | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $PWD "artifacts/run-config.json") -Encoding UTF8

& $chrome "--user-data-dir=$ChromeProfile" "--proxy-server=$ProxyUrl" $signIn.url | Out-Null
Start-Sleep -Seconds 2

node (Join-Path $PWD "scripts/cdp-captcha.mjs")
node (Join-Path $PWD "scripts/minimax-flow.mjs")
