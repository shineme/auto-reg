$ErrorActionPreference = "Continue"
Set-Location "C:\Users\dhl\Desktop\m3\go-protocol"
$env:MINIMAX_CURSOR_DIRECT = "1"
& ".\build\bin\minimax-cli.exe" -cursor-batch `
  -cursor-panel "http://web.open-traffic.com" `
  -cursor-key "cbe562d176daaffdc4fde0859dffce8599d4b69356b27e39" `
  -cursor-count $args[0] `
  2>&1 | Tee-Object -FilePath "cursor-batch.log"
