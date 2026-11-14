param(
  [Parameter(Mandatory = $true)][int]$Count,
  [string]$PanelKey = $env:PANEL_KEY,
  [string]$ImapUser = $env:IMAP_USER,
  [string]$ImapPassword = $env:IMAP_PASSWORD,
  [string]$BatchRoot = (Join-Path $PWD "artifacts/batch"),
  [string]$BatchId = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss"),
  [string]$Base = $(if ($env:PANEL_BASE) { $env:PANEL_BASE } else { "https://panel.example.com" }),
  [string]$ProxyUrl = "http://127.0.0.1:2080",
  [int]$BaseCdpPort = 9400,
  [int]$StartIndex = 1,
  [int]$DelaySeconds = 5
)

$ErrorActionPreference = "Stop"
if ($Count -lt 1) { throw "Count must be at least 1" }
if (!$PanelKey -or !$ImapUser -or !$ImapPassword) { throw "PANEL_KEY, IMAP_USER and IMAP_PASSWORD are required" }

$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$unitScript = Join-Path $workspace "scripts/run-minimax-batch-unit.ps1"
$root = Join-Path $BatchRoot $BatchId
New-Item -ItemType Directory -Path $root -Force | Out-Null
$summaryPath = Join-Path $root "summary.json"
$results = @()

for ($offset = 0; $offset -lt $Count; $offset += 1) {
  $index = $StartIndex + $offset
  $runDir = Join-Path $root ("account-{0:D3}" -f $index)
  $cdpPort = $BaseCdpPort + $index
  $startedAt = (Get-Date).ToUniversalTime().ToString("o")
  try {
    $output = & $unitScript -PanelKey $PanelKey -ImapUser $ImapUser -ImapPassword $ImapPassword -RunDir $runDir -CdpPort $cdpPort -Base $Base -ProxyUrl $ProxyUrl
    $result = $output | ConvertFrom-Json
    $results += [ordered]@{ index = $index; ok = $true; user = $result.user; email = $result.email; runDir = $runDir; startedAt = $startedAt }
    Write-Host ("[{0}/{1}] OK {2} {3}" -f ($offset + 1), $Count, $result.user, $result.email)
  } catch {
    $unitResultPath = Join-Path $runDir "result.json"
    $errorMessage = $_.Exception.Message
    if (Test-Path -LiteralPath $unitResultPath) {
      $unitResult = Get-Content -LiteralPath $unitResultPath -Raw | ConvertFrom-Json
      $errorMessage = $unitResult.error
    }
    $results += [ordered]@{ index = $index; ok = $false; error = $errorMessage; runDir = $runDir; startedAt = $startedAt }
    Write-Warning ("[{0}/{1}] FAIL {2}" -f ($offset + 1), $Count, $errorMessage)
  }
  $summary = [ordered]@{
    batchId = $BatchId
    requested = $Count
    completed = $results.Count
    succeeded = @($results | Where-Object { $_.ok }).Count
    failed = @($results | Where-Object { !$_.ok }).Count
    updatedAt = (Get-Date).ToUniversalTime().ToString("o")
    results = $results
  }
  $summary | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
  if ($offset + 1 -lt $Count -and $DelaySeconds -gt 0) { Start-Sleep -Seconds $DelaySeconds }
}

$summary = [ordered]@{
  batchId = $BatchId
  requested = $Count
  completed = $results.Count
  succeeded = @($results | Where-Object { $_.ok }).Count
  failed = @($results | Where-Object { !$_.ok }).Count
  finishedAt = (Get-Date).ToUniversalTime().ToString("o")
  results = $results
  summaryPath = $summaryPath
}
$summary | ConvertTo-Json -Depth 20 -Compress
