#Requires -Version 5.1
<#
.SYNOPSIS
  Builds the portable Windows app: dist\pgp-tools.exe

.DESCRIPTION
  Needs Go and a MinGW-w64 gcc in PATH (Fyne uses cgo). windres from the same
  MinGW install adds the icon, manifest (no admin prompt, per-monitor DPI,
  long paths) and version info; without it the exe still builds, without those.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1
  powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version v1.2.0 -SkipTests
#>
param(
    [string]$Version = "",
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)

function Require-Tool([string]$Name, [string]$Hint) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name not found in PATH. $Hint"
    }
}
Require-Tool 'go'  'Install Go from https://go.dev/dl/'
Require-Tool 'gcc' 'Install MinGW-w64, e.g. MSYS2 then: pacman -S mingw-w64-ucrt-x86_64-gcc, and add its bin folder to PATH.'

if (-not $Version) {
    $Version = (git describe --tags --always 2>$null)
    if (-not $Version) { $Version = 'dev' }
}
$m = [regex]::Match($Version, '(\d+)\.(\d+)\.(\d+)')
if ($m.Success) { $maj, $min, $pat = $m.Groups[1].Value, $m.Groups[2].Value, $m.Groups[3].Value }
else { $maj, $min, $pat = '0', '0', '0' }

if (Get-Command windres -ErrorAction SilentlyContinue) {
    Push-Location winres
    try {
        windres "-DVER_MAJOR=$maj" "-DVER_MINOR=$min" "-DVER_PATCH=$pat" -O coff -i pgp-tools.rc -o ..\cmd\pgp-tools\rsrc_windows_amd64.syso
        if ($LASTEXITCODE -ne 0) { throw "windres failed (exit $LASTEXITCODE)" }
    } finally { Pop-Location }
} else {
    Write-Warning 'windres not found: building without icon, manifest and version info.'
}

$env:CGO_ENABLED = '1'
if (-not $SkipTests) {
    go test -count=1 ./internal/... ./tools/...
    if ($LASTEXITCODE -ne 0) { throw "tests failed (exit $LASTEXITCODE)" }
}

New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=$Version" -o dist\pgp-tools.exe .\cmd\pgp-tools
if ($LASTEXITCODE -ne 0) { throw "build failed (exit $LASTEXITCODE)" }

Write-Host "Built dist\pgp-tools.exe ($Version). Double-click to run; use Tools > Install to Start Menu to install."
