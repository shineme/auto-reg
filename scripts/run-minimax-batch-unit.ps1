param(
  [Parameter(Mandatory = $true)][string]$PanelKey,
  [Parameter(Mandatory = $true)][string]$ImapUser,
  [Parameter(Mandatory = $true)][string]$ImapPassword,
  [Parameter(Mandatory = $true)][string]$RunDir,
  [Parameter(Mandatory = $true)][int]$CdpPort,
  [string]$Base = $(if ($env:PANEL_BASE) { $env:PANEL_BASE } else { "https://panel.example.com" }),
  [string]$MailDomain = $(if ($env:MAIL_DOMAIN) { $env:MAIL_DOMAIN } else { "example.com" }),
  [string]$ProxyUrl = "http://127.0.0.1:2080",
  [string]$ChromeProfile = "",
  [string]$ChromePath = "C:\Program Files\Google\Chrome\Application\chrome.exe",
  [int]$SignInPollSeconds = 90
)

$ErrorActionPreference = "Stop"

function Write-JsonFile([string]$Path, $Value) {
  $Value | ConvertTo-Json -Depth 30 -Compress | Set-Content -LiteralPath $Path -Encoding UTF8
}

if (!$ChromeProfile) { $ChromeProfile = Join-Path $RunDir "chrome-profile" }
$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$statePath = Join-Path $RunDir "session.json"
$configPath = Join-Path $RunDir "run-config.json"
$signInPath = Join-Path $RunDir "signin.json"
$flowPath = Join-Path $RunDir "flow.json"
$signInVerifyPath = Join-Path $RunDir "signin-verify.json"
$providersPath = Join-Path $RunDir "providers.json"
$resultPath = Join-Path $RunDir "result.json"

New-Item -ItemType Directory -Path $RunDir -Force | Out-Null
New-Item -ItemType Directory -Path $ChromeProfile -Force | Out-Null
Remove-Item -LiteralPath $statePath -ErrorAction SilentlyContinue

$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-WebRequest -Uri "$Base/?k=$([uri]::EscapeDataString($PanelKey))&view=providers&edit=minimax-code-global" -WebSession $session -UseBasicParsing -TimeoutSec 20 | Out-Null

$email = "mm$([guid]::NewGuid().ToString('N').Substring(0,10))@$MailDomain"
$password = "Mm!$([guid]::NewGuid().ToString('N'))"
$body = '{"provider":"minimax-code-global","method":0,"inputs":{},"proxy":""}'
$signIn = Invoke-RestMethod -Uri "$Base/api/plugin-signin" -WebSession $session -Method Post -Headers @{ "Content-Type" = "application/json" } -Body $body -TimeoutSec 30
Write-JsonFile $signInPath $signIn

$loginRedirect = ([uri]$signIn.url).PathAndQuery
$config = @{
  userCode = $signIn.code
  email = $email
  password = $password
  loginRedirect = $loginRedirect
  createdAt = (Get-Date).ToUniversalTime().ToString("o")
}
Write-JsonFile $configPath $config

$env:PANEL_BASE = $Base
$env:MINIMAX_PROXY_URL = $ProxyUrl
$env:STATE_PATH = $statePath
$env:RUN_CONFIG_PATH = $configPath
$env:FLOW_PATH = $flowPath
$env:USER_CODE = $signIn.code
$env:EMAIL = $email
$env:PASSWORD = $password
$env:LOGIN_REDIRECT = $loginRedirect
$env:CAPTCHA_ACTION = "send_code"
$env:IMAP_USER = $ImapUser
$env:IMAP_PASSWORD = $ImapPassword
$env:CDP_PORT = $CdpPort
$env:CDP_HOST = "127.0.0.1"
$env:CDP_TARGET_URL = $signIn.url

$chromeArgs = @(
  "--remote-debugging-port=$CdpPort",
  "--user-data-dir=$ChromeProfile",
  "--no-first-run",
  "--no-default-browser-check",
  "--disable-background-networking",
  "--window-position=-32000,-32000",
  "--window-size=3440,1440",
  "--proxy-server=$ProxyUrl",
  "about:blank"
)
& $ChromePath @chromeArgs | Out-Null

try {
  $deadline = (Get-Date).AddSeconds(20)
  $cdpReady = $false
  while ((Get-Date) -lt $deadline) {
    try {
      Invoke-RestMethod -Uri "http://127.0.0.1:$CdpPort/json/version" -TimeoutSec 2 | Out-Null
      $cdpReady = $true
      break
    } catch {
      Start-Sleep -Milliseconds 300
    }
  }
  if (!$cdpReady) { throw "CDP port $CdpPort did not become ready" }

  & $ChromePath "--user-data-dir=$ChromeProfile" "--proxy-server=$ProxyUrl" $signIn.url | Out-Null
  Start-Sleep -Seconds 2
  node (Join-Path $workspace "scripts/cdp-captcha.mjs") | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "cdp-captcha.mjs failed with exit code $LASTEXITCODE" }
  node (Join-Path $workspace "scripts/minimax-flow.mjs")
  if ($LASTEXITCODE -ne 0) { throw "minimax-flow.mjs failed with exit code $LASTEXITCODE" }

  $flow = Get-Content -LiteralPath $flowPath -Raw | ConvertFrom-Json
  if ($flow.status -ne "approved") { throw "MiniMax flow status is $($flow.status)" }

  $signInDone = $null
  $pollDeadline = (Get-Date).AddSeconds($SignInPollSeconds)
  while ((Get-Date) -lt $pollDeadline) {
    $signInDone = Invoke-RestMethod -Uri "$Base/api/signin/$($signIn.id)" -WebSession $session -TimeoutSec 20
    if ($signInDone.state -eq "done" -or $signInDone.state -eq "failed" -or $signInDone.state -eq "error") { break }
    Start-Sleep -Seconds 2
  }
  Write-JsonFile $signInVerifyPath $signInDone
  if ($signInDone.state -ne "done") { throw "magpie sign-in state is $($signInDone.state)" }

  $providers = Invoke-RestMethod -Uri "$Base/api/providers" -WebSession $session -TimeoutSec 30
  Write-JsonFile $providersPath $providers
  $providersText = $providers | ConvertTo-Json -Depth 30 -Compress
  if (!$providersText.Contains([string]$signInDone.user)) { throw "providers API does not list $($signInDone.user)" }

  $result = [ordered]@{
    ok = $true
    email = $email
    password = $password
    user = $signInDone.user
    userCode = $signIn.code
    signInId = $signIn.id
    flowStatus = $flow.status
    signInState = $signInDone.state
    runDir = $RunDir
    completedAt = (Get-Date).ToUniversalTime().ToString("o")
  }
  Write-JsonFile $resultPath $result
  $result | ConvertTo-Json -Compress
} catch {
  $result = [ordered]@{
    ok = $false
    email = $email
    password = $password
    userCode = $signIn.code
    signInId = $signIn.id
    error = $_.Exception.Message
    runDir = $RunDir
    completedAt = (Get-Date).ToUniversalTime().ToString("o")
  }
  Write-JsonFile $resultPath $result
  throw
} finally {
  Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" |
    Where-Object { $_.CommandLine -like "*--user-data-dir=$ChromeProfile*" } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}
