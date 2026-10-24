Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public class WinList {
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lp);
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lp);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, [MarshalAs(UnmanagedType.LPWStr)] StringBuilder sb, int max);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr hWnd, [MarshalAs(UnmanagedType.LPWStr)] StringBuilder sb, int max);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
'@

$target = (Get-Process forge-desktop-console -ErrorAction SilentlyContinue | Select-Object -First 1).Id
if (-not $target) { $target = (Get-Process forge-desktop -ErrorAction SilentlyContinue | Select-Object -First 1).Id }
if (-not $target) { Write-Output "NO_PROCESS"; exit }
Write-Output "process: $target"
$cb = {
  param($h, $lp)
  $procId = [uint32]0
  [WinList]::GetWindowThreadProcessId($h, [ref]$procId) | Out-Null
  if ($procId -eq $target) {
    $t = New-Object System.Text.StringBuilder 256
    [WinList]::GetWindowText($h, $t, 256) | Out-Null
    $c = New-Object System.Text.StringBuilder 256
    [WinList]::GetClassName($h, $c, 256) | Out-Null
    $r = New-Object WinList+RECT
    [WinList]::GetWindowRect($h, [ref]$r) | Out-Null
    $vis = [WinList]::IsWindowVisible($h)
    Write-Output ("hwnd=0x{0:x} [{1}] class={2} {3}x{4} vis={5}" -f $h, $t.ToString(), $c.ToString(), ($r.Right-$r.Left), ($r.Bottom-$r.Top), $vis)
  }
  return $true
}
[WinList]::EnumWindows($cb, [IntPtr]::Zero) | Out-Null
