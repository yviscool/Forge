Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public class WinCount {
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lp);
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lp);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
  [DllImport("user32.dll")] public static extern int GetWindowTextW(IntPtr hWnd, StringBuilder sb, int max);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
  [DllImport("user32.dll")] public static extern int GetWindowLongW(IntPtr hWnd, int index);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
'@

$target = (Get-Process forge-desktop -ErrorAction SilentlyContinue | Select-Object -First 1).Id
if (-not $target) { Write-Output "NO_PROCESS"; exit }
Write-Output "process: $target"
$cb = {
  param($h, $lp)
  $procId = 0
  [WinCount]::GetWindowThreadProcessId($h, [ref]$procId) | Out-Null
  if ($procId -eq $target) {
    $sb = New-Object System.Text.StringBuilder 256
    [WinCount]::GetWindowTextW($h, $sb, 256) | Out-Null
    $title = $sb.ToString()
    if ($title.Length -gt 0) {
      $r = New-Object WinCount+RECT
      [WinCount]::GetWindowRect($h, [ref]$r) | Out-Null
      $vis = [WinCount]::IsWindowVisible($h)
      $ex = [WinCount]::GetWindowLongW($h, -20)
      $tool = ($ex -band 0x80) -ne 0
      Write-Output ("win [{0}] {1}x{2} visible={3} toolwindow={4}" -f $title, ($r.Right-$r.Left), ($r.Bottom-$r.Top), $vis, $tool)
    }
  }
  return $true
}
[WinCount]::EnumWindows($cb, [IntPtr]::Zero) | Out-Null
