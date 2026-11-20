# 只编译命令行版，输出到单独文件名，避免覆盖 Wails 打包的 GUI 主程序。
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
go build -o build\bin\minimax-cli.exe .
Write-Host "CLI 已生成: build\bin\minimax-cli.exe"
Write-Host "GUI 请用: wails build  (输出 build\bin\minimax-go-protocol.exe)"
