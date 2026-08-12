param(
    [string]$ImageName = $(if ($env:IMAGE_NAME) { $env:IMAGE_NAME } else { "jialtang/octopus" }),
    [string]$ImageTag = $(if ($env:IMAGE_TAG) { $env:IMAGE_TAG } else { "latest" }),
    [string]$TargetPlatform = $(if ($env:TARGET_PLATFORM) { $env:TARGET_PLATFORM } else { "linux/amd64" }),
    [switch]$ExportImage,
    [string]$OutputDir = $(if ($env:OUTPUT_DIR) { $env:OUTPUT_DIR } else { "build" }),
    [string]$Dockerfile = $(if ($env:DOCKERFILE) { $env:DOCKERFILE } else { "scripts/dockerfiles/Dockerfile.alpine" })
)

$ErrorActionPreference = "Stop"

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $RootDir

$goArm = $null
switch ($TargetPlatform) {
    "linux/amd64" { $goOs = "linux"; $goArch = "amd64" }
    "linux/arm64" { $goOs = "linux"; $goArch = "arm64" }
    "linux/386" { $goOs = "linux"; $goArch = "386" }
    "linux/arm/v7" { $goOs = "linux"; $goArch = "arm"; $goArm = "7" }
    default { throw "Unsupported TargetPlatform: $TargetPlatform" }
}

$Version = if ($env:VERSION) { $env:VERSION } else {
    $tag = git describe --tags --abbrev=0 2>$null
    if ($LASTEXITCODE -eq 0 -and $tag) { $tag } else { "dev" }
}
$ImageRef = "${ImageName}:${ImageTag}"
$SafeImageName = $ImageName -replace "/", "-"
$AssetName = if ($env:ASSET_NAME) { $env:ASSET_NAME } else { "Docker-${SafeImageName}-${ImageTag}.tar.gz" }

Write-Host "Building frontend..."
Push-Location web
pnpm install --frozen-lockfile
$env:NEXT_PUBLIC_APP_VERSION = $Version
pnpm run build
Pop-Location

Remove-Item "static/out" -Recurse -Force -ErrorAction SilentlyContinue
Move-Item -Force "web/out" "static/out"

Write-Host "Updating price data..."
# python scripts/updatePrice.py

Write-Host "Building Go backend for $TargetPlatform..."
$BinaryDir = Join-Path $OutputDir "docker/$TargetPlatform"
New-Item -ItemType Directory -Force $BinaryDir | Out-Null

$env:GOOS = $goOs
$env:GOARCH = $goArch
$env:CGO_ENABLED = "0"
if ($goArm) {
    $env:GOARM = $goArm
} else {
    Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
}
go build -o (Join-Path $BinaryDir "octopus") -ldflags="-s -w" -tags=jsoniter .

Write-Host "Building Docker image $ImageRef..."
docker build `
    --pull `
    --build-arg "TARGETPLATFORM=$TargetPlatform" `
    -f $Dockerfile `
    -t $ImageRef `
    .

if ($ExportImage) {
    Write-Host "Exporting Docker image to $AssetName..."
    docker save $ImageRef -o "$AssetName.tmp"
    if (Test-Path $AssetName) {
        Remove-Item $AssetName -Force
    }
    gzip -9 "$AssetName.tmp"
    Move-Item -Force "$AssetName.tmp.gz" $AssetName
    gzip -t $AssetName
    Get-Item $AssetName | Format-List Name,Length
}

Write-Host "Docker image ready: $ImageRef"
