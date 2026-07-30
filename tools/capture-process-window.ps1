param(
    [Parameter(Mandatory = $true, ParameterSetName = "Process")]
    [int]$ProcessId,

    [Parameter(Mandatory = $true, ParameterSetName = "Window")]
    [Int64]$WindowHandle,

    [Parameter(Mandatory = $true)]
    [string]$Destination
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;

public static class WindowCaptureNative
{
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT
    {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);

    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hWnd);

    [DllImport("user32.dll")]
    public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint flags);
}
"@

$handle = if ($PSCmdlet.ParameterSetName -eq "Window") {
    [IntPtr]::new($WindowHandle)
}
else {
    (Get-Process -Id $ProcessId).MainWindowHandle
}
if ($handle -eq [IntPtr]::Zero) {
    throw "The selected process or window does not have a usable window handle"
}
if (-not [WindowCaptureNative]::IsWindowVisible($handle)) {
    throw "Window $handle is hidden; use capture-electron-page.js instead"
}

$rect = New-Object WindowCaptureNative+RECT
if (-not [WindowCaptureNative]::GetWindowRect($handle, [ref]$rect)) {
    throw "GetWindowRect failed for handle $handle"
}

$width = $rect.Right - $rect.Left
$height = $rect.Bottom - $rect.Top
if ($width -le 0 -or $height -le 0) {
    throw "Invalid window size ${width}x${height}"
}

$destinationPath = [System.IO.Path]::GetFullPath($Destination)
$destinationDirectory = [System.IO.Path]::GetDirectoryName($destinationPath)
[System.IO.Directory]::CreateDirectory($destinationDirectory) | Out-Null

$bitmap = New-Object System.Drawing.Bitmap(
    $width,
    $height,
    [System.Drawing.Imaging.PixelFormat]::Format32bppArgb
)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
try {
    $deviceContext = $graphics.GetHdc()
    try {
        $captured = [WindowCaptureNative]::PrintWindow($handle, $deviceContext, 2)
    }
    finally {
        $graphics.ReleaseHdc($deviceContext)
    }

    if (-not $captured) {
        $graphics.CopyFromScreen(
            $rect.Left,
            $rect.Top,
            0,
            0,
            [System.Drawing.Size]::new($width, $height)
        )
    }

    $bitmap.Save($destinationPath, [System.Drawing.Imaging.ImageFormat]::Png)
}
finally {
    $graphics.Dispose()
    $bitmap.Dispose()
}

[pscustomobject]@{
    ProcessId = if ($PSCmdlet.ParameterSetName -eq "Process") { $ProcessId } else { $null }
    WindowHandle = ("0x{0:X}" -f $handle.ToInt64())
    Destination = $destinationPath
    Width = $width
    Height = $height
    PrintWindow = $captured
} | ConvertTo-Json
