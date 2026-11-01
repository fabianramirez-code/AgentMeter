# Usage:
#   .\build.ps1            build and install into an installed Rainmeter (Documents\Rainmeter\Skins)
#   .\build.ps1 -Portable  build, download a portable Rainmeter into .\rainmeter (first run only),
#                          install the skin there and launch it
param([switch]$Portable)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot

# Use Go from PATH; otherwise use (or download once) a project-local copy in
# .\go (git-ignored). It is the official zip, checksum-verified: no installer,
# no admin rights, no system PATH changes.
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
$localGo = Join-Path $root "go\bin\go.exe"
if (-not $go -and -not (Test-Path $localGo)) {
    Write-Host "Go not found, downloading a project-local copy..."
    $release = (Invoke-RestMethod "https://go.dev/dl/?mode=json")[0]
    $file = $release.files | Where-Object { $_.os -eq "windows" -and $_.arch -eq "amd64" -and $_.kind -eq "archive" } | Select-Object -First 1
    $zip = Join-Path $env:TEMP $file.filename
    Invoke-WebRequest "https://go.dev/dl/$($file.filename)" -OutFile $zip
    if ((Get-FileHash $zip -Algorithm SHA256).Hash -ne $file.sha256.ToUpper()) {
        Remove-Item $zip
        throw "Go download checksum mismatch; aborting."
    }
    Expand-Archive $zip -DestinationPath $root # the zip contains a top-level "go" folder
    Remove-Item $zip
}
if (-not $go) { $go = $localGo }
$skinSrc = Join-Path $root "skin\AgentMeter"
$exePath = Join-Path $skinSrc "@Resources\agentmeter.exe"
$portableDir = Join-Path $root "rainmeter"
$rainmeterExe = Join-Path $portableDir "Rainmeter.exe"

if ($Portable) {
    $skinsDir = Join-Path $portableDir "Skins\AgentMeter"
    if (-not (Test-Path $rainmeterExe)) {
        # Official installer from GitHub releases, run in silent portable mode:
        # program, skins and settings all stay inside .\rainmeter (git-ignored),
        # no registry or Start menu changes.
        Write-Host "Downloading portable Rainmeter..."
        $release = Invoke-RestMethod "https://api.github.com/repos/rainmeter/rainmeter/releases/latest"
        $asset = $release.assets | Where-Object { $_.name -like "Rainmeter-*.exe" } | Select-Object -First 1
        $installer = Join-Path $env:TEMP $asset.name
        Invoke-WebRequest $asset.browser_download_url -OutFile $installer
        if ((Get-AuthenticodeSignature $installer).Status -ne "Valid") {
            throw "Rainmeter installer signature is not valid; aborting."
        }
        # /D= must be the last argument and must not be quoted.
        Start-Process $installer -ArgumentList "/S", "/PORTABLE=1", "/D=$portableDir" -Wait
        Remove-Item $installer
        if (-not (Test-Path $rainmeterExe)) { throw "Portable Rainmeter install failed." }
    }
} else {
    $skinsDir = Join-Path $env:USERPROFILE "Documents\Rainmeter\Skins\AgentMeter"
}

Write-Host "Running tests..."
& $go test ./...
if ($LASTEXITCODE -ne 0) { throw "go test failed" }

# -trimpath keeps local paths (user name, module cache) out of the binary.
Write-Host "Building agentmeter.exe..."
& $go build -trimpath -ldflags "-s -w -H windowsgui" -o $exePath .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

Write-Host "Installing skin to $skinsDir..."
New-Item -ItemType Directory -Force -Path $skinsDir | Out-Null

# Copy everything except the user's own data.json/state.json, so a
# reinstall never clobbers their cached usage data.
#
# .ini/.inc files are kept as plain UTF-8 in the repo (diff-friendly) but
# Rainmeter reads skin .ini files as ANSI unless they carry a UTF-16 LE
# BOM, which mangles any non-ASCII glyph (▲ ▼ ⚙ render as "â–²â–¼âš™").
# So convert them to UTF-16 LE + BOM here, at install time, instead of
# storing that encoding in git. Lua scripts are left as UTF-8/ASCII;
# Rainmeter's Lua plugin does not have this requirement.
$utf16 = New-Object System.Text.UnicodeEncoding($false, $true) # LE, with BOM
Get-ChildItem -Path $skinSrc -Recurse | ForEach-Object {
    $relative = $_.FullName.Substring($skinSrc.Length + 1)
    $dest = Join-Path $skinsDir $relative
    if ($_.PSIsContainer) {
        New-Item -ItemType Directory -Force -Path $dest | Out-Null
        return
    }
    if ($_.Name -eq "data.json" -or $_.Name -eq "state.json") {
        return
    }
    if ($_.Extension -eq ".ini" -or $_.Extension -eq ".inc") {
        $text = Get-Content -Path $_.FullName -Raw -Encoding UTF8
        [System.IO.File]::WriteAllText($dest, $text, $utf16)
        return
    }
    Copy-Item -Path $_.FullName -Destination $dest -Force
}

if ($Portable) {
    Write-Host "Launching portable Rainmeter with AgentMeter..."
    Start-Process $rainmeterExe
    Start-Sleep -Seconds 3
    & $rainmeterExe "!ActivateConfig" "AgentMeter" "AgentMeter.ini"
    Write-Host "Done. Run .\rainmeter\Rainmeter.exe to start it again later."
} else {
    Write-Host "Done. Load AgentMeter in Rainmeter to start it."
}
