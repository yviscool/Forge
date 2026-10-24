@echo off
chcp 65001 >nul
title Forge 教师台启动器

rem ============================================================
rem  Forge 桌面模式启动器（教师机双击用）
rem  数据目录固定为 data\demo，管理员密码内置，避免
rem  「直接双击 exe 用错数据目录导致登录不上」的问题。
rem ============================================================

cd /d "%~dp0"

set FORGE_DATA_DIR=C:\Users\Administrator\Desktop\Forge\data\demo
set FORGE_ADMIN_PASSWORD=Forge-Test-2026!
set FORGE_AUTOJUDGE=true
set FORGE_ALLOW_UNSAFE_LOCAL_JUDGE=1
set FORGE_ADDR=0.0.0.0:8080

echo.
echo   正在启动 Forge 教师台...
echo   登录账号: admin    密码: Forge-Test-2026!
echo   学生访问: http://本机IP:8080/app （启动后托盘左键可看扫码页）
echo.

forge-desktop.exe %*
