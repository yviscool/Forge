# 教师机启动器（PowerShell 版）：与「启动教师台.bat」等价，但可在
# 计划任务 / 无控制台环境里使用（cmd 的 set 在部分宿主里不可靠）。
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\devtools\start-teacher.ps1
#
# 需要固定数据目录时先设 FORGE_DATA_DIR 再调用（默认 ./data）。
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $root

if (-not $env:FORGE_ADMIN_PASSWORD) { $env:FORGE_ADMIN_PASSWORD = 'Forge-Test-2026!' }
$env:FORGE_AUTOJUDGE = 'true'
$env:FORGE_ALLOW_UNSAFE_LOCAL_JUDGE = '1'
if (-not $env:FORGE_ADDR) { $env:FORGE_ADDR = '0.0.0.0:8080' }

Write-Host "Forge 教师台: $env:FORGE_ADDR  (数据目录 $root\data)"
Write-Host "登录账号 admin / admin"
# 无控制台启动时 stdout 也要落地，否则排查「接口报错」时看不到请求日志。
$log = Join-Path $root 'data\desktop-stdout.log'
& (Join-Path $root 'forge-desktop.exe') *>> $log
