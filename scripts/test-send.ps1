param(
    [string]$TestType = "primary"
)

Set-Location (Resolve-Path "$PSScriptRoot\..")

switch ($TestType) {
    "primary" {
        $ServerArgs = "receive --file test\out.txt"
        $ClientArgs = "send --file test\in.txt --points $(Get-Random -Minimum 4 -Maximum 48)"
    }
    "secondary" {
        $ServerArgs = "send --file test\in.txt --points $(Get-Random -Minimum 4 -Maximum 60)"
        $ClientArgs = "receive --file test\out.txt"
    }
    default {
        Write-Host "Usage: $($MyInvocation.MyCommand.Name) {primary|secondary}"
        exit 1
    }
}

function Get-Executable {
    $path = "dist/dingopie_windows_amd64/dingopie.exe"
    if ($env:EXECUTABLE -and $env:EXECUTABLE.Trim() -ne "") {
        $path = $env:EXECUTABLE
    }
    return $path.Replace("/", "\")
}

function Write-RandomBase64File {
    param(
        [string]$Path,
        [int]$NumBytes
    )
    $bytes = New-Object byte[] $NumBytes
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $content = [Convert]::ToBase64String($bytes)
    [System.IO.File]::WriteAllText($Path, $content, [System.Text.Encoding]::ASCII)
}

function Split-Args {
    param([string]$InputString)
    if ([string]::IsNullOrWhiteSpace($InputString)) { return @() }
    return $InputString -split '\s+' | Where-Object { $_ -ne "" }
}

function Get-FileSha256 {
    param([string]$Path)
    try {
        $sha256 = [System.Security.Cryptography.SHA256]::Create()
        $stream = [System.IO.File]::OpenRead($Path)
        $hash = $sha256.ComputeHash($stream)
        $stream.Close()
        return [BitConverter]::ToString($hash).Replace("-", "")
    } catch {
        Write-Error "Failed to calculate hash for $Path : $_"
        return $null
    }
}

$exe = Get-Executable

Write-Host "==> Starting test"

if (Test-Path .\test) {
    Remove-Item -Recurse -Force .\test
}
New-Item -ItemType Directory .\test | Out-Null

Write-RandomBase64File ".\test\in.txt"  (Get-Random -Minimum 256 -Maximum 8193)
Write-RandomBase64File ".\test\key.txt" (Get-Random -Minimum 8   -Maximum 33)

Write-Host "--> Starting server in background"
$key = Get-Content -Raw .\test\key.txt
$serverCmd = "$exe server direct $ServerArgs --key `"$key`" > `".\test\server.log`" 2>&1"
$serverProc = Start-Process cmd.exe -ArgumentList "/c", $serverCmd -PassThru
Start-Sleep 1

Write-Host "--> Starting client"
$waitMs = Get-Random -Minimum 10 -Maximum 501

$clientArgs =
    @("client","direct") +
    (Split-Args $ClientArgs) +
    @("--key",$key,"--server-ip","127.0.0.1","--wait","${waitMs}ms")

& $exe @clientArgs
$clientRc = $LASTEXITCODE

Start-Sleep 1

if ($serverProc -and -not $serverProc.HasExited) {
    Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    Write-Host "--> Server stopped by force (unexpected)"
} else {
    Write-Host "--> Server already stopped on its own (expected)"
}

Write-Host "--> Server log:`n"
if (Test-Path .\test\server.log) {
    Get-Content .\test\server.log -Raw
}

Write-Host "`n--> Verifying outputs match"

if (-not (Test-Path .\test\out.txt)) {
    Write-Host "==> FAILED"
    Remove-Item -Recurse -Force .\test
    exit 1
}

$h1 = Get-FileSha256 ".\test\in.txt"
$h2 = Get-FileSha256 ".\test\out.txt"

if ($null -ne $h1 -and $h1 -eq $h2) {
    Write-Host "==> PASSED"
    $rc = 0
} else {
    Write-Host "==> FAILED"
    Write-Host "Hash 1: $h1"
    Write-Host "Hash 2: $h2"
    $rc = 1
}

Write-Host "--> Cleaning up"
Remove-Item -Recurse -Force .\test

Write-Host "==> Complete"

Start-Sleep 1
exit $rc
