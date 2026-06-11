param(
  [string]$OutputDir = "release\go-fontman",
  [switch]$Clean
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

function Info([string]$Message) {
  Write-Host "[go-fontman] $Message"
}

function Copy-Directory([string]$Source, [string]$Destination) {
  if (-not (Test-Path -LiteralPath $Source)) {
    throw "Missing required directory: $Source"
  }
  if (Test-Path -LiteralPath $Destination) {
    Remove-Item -LiteralPath $Destination -Recurse -Force
  }
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
  Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
}

function Assert-File([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "Missing required file: $Path"
  }
}

$outputPath = if ([System.IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir
} else {
  Join-Path $repoRoot $OutputDir
}

if ($Clean -and (Test-Path -LiteralPath $outputPath)) {
  Info "Cleaning $outputPath"
  Remove-Item -LiteralPath $outputPath -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null

Info "Checking runtime assets"
Assert-File "runtime\font_ai\manifest.json"
Assert-File "runtime\font_ai\model\font_embedding.onnx"
Assert-File "runtime\font_ai\model\font_embedding.onnx.data"
Assert-File "runtime\font_ai\data\font_index.f32bin"
Assert-File "runtime\font_ai\data\font_index_meta.json"
Assert-File "runtime\onnxruntime\win-x64\onnxruntime.dll"
Assert-File "runtime\onnxruntime\win-x64\onnxruntime_providers_shared.dll"

Info "Running tests"
go test ./...

Info "Building fontman-cli.exe"
go build -trimpath -ldflags="-s -w" -o (Join-Path $outputPath "fontman-cli.exe") .\cmd\fontman-cli

Info "Building fontman-service.exe"
go build -trimpath -ldflags="-s -w" -o (Join-Path $outputPath "fontman-service.exe") .\cmd\fontman-service

Info "Copying runtime assets"
Copy-Directory "runtime" (Join-Path $outputPath "runtime")

Info "Writing release README"
Copy-Item -LiteralPath "scripts\release_README.md" -Destination (Join-Path $outputPath "README.md") -Force

Info "Release ready: $outputPath"
