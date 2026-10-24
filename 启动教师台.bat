@echo off
chcp 65001 >nul
title Forge 教师台启动器

rem ============================================================
rem  Forge 桌面模式启动器（教师机双击用）
rem  数据目录用默认的 data\（与直接双击 forge-desktop.exe 相同，
rem  会话互通，不再需要指定演示目录）。
rem ============================================================

cd /d "%~dp0"

set FORGE_ADMIN_PASSWORD=Forge-Test-2026!
set FORGE_AUTOJUDGE=true
set FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1
set FORGE_ADDR=0.0.0.0:8080

echo.
echo   正在启动 Forge 教师台...
echo   登录账号: admin    密码: admin
echo   （FORGE_ADMIN_PASSWORD 只在 data 目录全新、需要创建 admin 时生效；
echo     当前库里的 admin 口令已改为 admin，由 scripts/devtools/setpass 设置）
echo   学生访问: http://本机IP:8080/app （托盘左键可看扫码页）
echo.

forge-desktop.exe %*
