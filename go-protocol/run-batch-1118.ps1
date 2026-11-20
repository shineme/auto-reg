$ErrorActionPreference = "Continue"
$dir = "C:\Users\dhl\Desktop\m3\artifacts\go-protocol-1118"
$exe = "C:\Users\dhl\Desktop\m3\go-protocol\build\bin\minimax-cli.exe"
$poolPath = "C:\Users\dhl\Desktop\m3\artifacts\go-protocol\hotmail-pool-1118.json"
$target = 1000
$supLog = Join-Path $dir "supervisor.log"

function Read-Summary {
  try { return (Get-Content (Join-Path $dir "summary.json") -Raw -Encoding UTF8) | ConvertFrom-Json } catch { return $null }
}
function Read-Pool {
  try { return (Get-Content $poolPath -Raw -Encoding UTF8) | ConvertFrom-Json } catch { return $null }
}

for ($i = 1; $i -le 500; $i++) {
  $sum = Read-Summary
  if ($sum -ne $null -and $sum.succeeded -ge $target) { Add-Content $supLog ((Get-Date -Format "HH:mm:ss") + " target reached " + $sum.succeeded + "/" + $target); break }
  $pool = Read-Pool
  $avail = 0
  $reserved = 0
  if ($pool -ne $null) {
    foreach ($a in $pool.accounts) {
      if ($a.status -eq "available" -or [string]::IsNullOrWhiteSpace($a.status)) { $avail = $avail + 1 }
      elseif ($a.status -eq "reserved") { $reserved = $reserved + 1 }
    }
  } else {
    $avail = 1
    Add-Content $supLog ((Get-Date -Format "HH:mm:ss") + " WARN pool read failed, keep going")
  }
  if ($avail -eq 0 -and $reserved -eq 0) { Add-Content $supLog ((Get-Date -Format "HH:mm:ss") + " pool exhausted"); break }
  if (Test-Path (Join-Path $dir "batch-run.log")) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    Move-Item (Join-Path $dir "batch-run.log") (Join-Path $dir ("batch-run." + $stamp + ".log")) -Force -ErrorAction SilentlyContinue
  }
  $okNow = 0
  if ($sum -ne $null) { $okNow = $sum.succeeded }
  Add-Content $supLog ((Get-Date -Format "HH:mm:ss") + " start #" + $i + " ok=" + $okNow + " avail=" + $avail + " reserved=" + $reserved)
  $p = Start-Process -FilePath $exe -ArgumentList "-batch","-count","$target","-workers","1" -RedirectStandardOutput (Join-Path $dir "batch-run.log") -RedirectStandardError (Join-Path $dir "batch-run.err.log") -WindowStyle Hidden -PassThru
  $p.Id | Set-Content (Join-Path $dir "run.pid")
  $p.WaitForExit()
  Add-Content $supLog ((Get-Date -Format "HH:mm:ss") + " exit code=" + $p.ExitCode)
  Start-Sleep -Seconds 15
}
