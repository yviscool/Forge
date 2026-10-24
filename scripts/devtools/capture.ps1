Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public class WinEnum {
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lp);
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lp);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
  [DllImport("user32.dll")] public static extern int GetWindowTextW(IntPtr hWnd, StringBuilder sb, int max);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdc, uint flags);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
'@

$target = (Get-Process forge-desktop -ErrorAction SilentlyContinue | Select-Object -First 1).Id
if (-not $target) { Write-Output "NO_PROCESS"; exit }

$script:best = [IntPtr]::Zero
$script:bestTitle = ""
$cb = {
  param($h, $lp)
  $procId = 0
  [WinEnum]::GetWindowThreadProcessId($h, [ref]$procId) | Out-Null
  if ($procId -eq $target -and [WinEnum]::IsWindowVisible($h)) {
    $sb = New-Object System.Text.StringBuilder 256
    [WinEnum]::GetWindowTextW($h, $sb, 256) | Out-Null
    $title = $sb.ToString()
    if ($title.Length -gt 0) {
      $r2 = New-Object WinEnum+RECT
      [WinEnum]::GetWindowRect($h, [ref]$r2) | Out-Null
      $ww = $r2.Right - $r2.Left
      Write-Output ("window: [{0}] ${ww}px" -f $title)
      $isTeacher = $title -like "*教师台*"
      if ($isTeacher) { $script:best = $h; $script:bestTitle = $title }
      elseif ($script:best -eq [IntPtr]::Zero -and $ww -gt 500) { $script:best = $h; $script:bestTitle = $title }
    }
  }
  return $true
}
[WinEnum]::EnumWindows($cb, [IntPtr]::Zero) | Out-Null

if ($script:best -eq [IntPtr]::Zero) {
  Write-Output "NO_TITLED_VISIBLE_WINDOW (main window likely hidden to tray; use tray or show-main)"
  exit
}

$r = New-Object WinEnum+RECT
[WinEnum]::GetWindowRect($script:best, [ref]$r) | Out-Null
$w = $r.Right - $r.Left; $h = $r.Bottom - $r.Top
if ($w -lt 10 -or $h -lt 10) { Write-Output "ZERO_SIZE_WINDOW ${w}x${h}"; exit }
$bmp = New-Object System.Drawing.Bitmap($w, $h)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$hdc = $g.GetHdc()
[WinEnum]::PrintWindow($script:best, $hdc, 2) | Out-Null
$g.ReleaseHdc($hdc); $g.Dispose()
$bmp.Save("C:\Users\Administrator\Desktop\Forge\window-shot.png")
Write-Output "SHOT ${w}x${h} TITLE=[$script:bestTitle]"
