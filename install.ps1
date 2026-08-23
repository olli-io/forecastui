# Install forecastui on Windows into %LOCALAPPDATA%\Programs\forecastui.
#
#   irm https://raw.githubusercontent.com/olli-io/forecastui/main/install.ps1 | iex
#
# Piped into iex there is nowhere to put parameters, so the knobs are read from
# the environment: FORECASTUI_VERSION pins a tag, FORECASTUI_BINDIR the target.

$ErrorActionPreference = 'Stop'

$repo    = 'olli-io/forecastui'
$version = $env:FORECASTUI_VERSION
$binDir  = $env:FORECASTUI_BINDIR
if (-not $binDir) { $binDir = Join-Path $env:LOCALAPPDATA 'Programs\forecastui' }

# Windows PowerShell 5.1 still defaults to TLS 1.0, which github.com refuses.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

# The archives are named after GOARCH. PROCESSOR_ARCHITECTURE reports the
# process, not the machine, so the *W6432 variant corrects an x86 host process.
$machine = $env:PROCESSOR_ARCHITEW6432
if (-not $machine) { $machine = $env:PROCESSOR_ARCHITECTURE }
switch ($machine) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "unsupported architecture: $machine" }
}

if (-not $version) {
    Write-Host 'looking up the latest release...'
    $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" `
        -Headers @{ 'User-Agent' = 'forecastui-install' }).tag_name
}
if (-not $version) { throw 'could not determine the latest release' }

$name = "forecastui_$($version.TrimStart('v'))_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/$version"
$tmp  = Join-Path ([IO.Path]::GetTempPath()) ("forecastui-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "downloading $name..."
    $zip = Join-Path $tmp $name
    Invoke-WebRequest "$base/$name" -OutFile $zip -UseBasicParsing

    # One SHA256SUMS covers every archive; this one's line is picked by name.
    try {
        $sums = (Invoke-WebRequest "$base/SHA256SUMS" -UseBasicParsing).Content
    } catch {
        $sums = $null
        Write-Warning 'SHA256SUMS not published, skipping checksum'
    }
    if ($sums) {
        $want = $null
        foreach ($line in ($sums -split "`n")) {
            $parts = $line.Trim() -split '\s+', 2
            if ($parts.Count -eq 2 -and $parts[1].TrimStart('*') -eq $name) { $want = $parts[0] }
        }
        if (-not $want) { throw "no checksum listed for $name" }
        $got = (Get-FileHash $zip -Algorithm SHA256).Hash
        if ($got -ne $want.ToUpper()) { throw "checksum mismatch for $name" }
    }

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null

    # Overwriting a running binary fails, so the new one lands beside the
    # target and is moved over it.
    $target = Join-Path $binDir 'forecastui.exe'
    $staged = "$target.new"
    Move-Item (Join-Path $tmp 'forecastui.exe') $staged -Force
    Move-Item $staged $target -Force
    Write-Host "installed $target"
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# The user-scoped PATH needs no elevation and survives a reboot; the current
# session is updated too, so forecastui runs straight away.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $binDir) {
    $updated = if ($userPath) { "$($userPath.TrimEnd(';'));$binDir" } else { $binDir }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    Write-Host "added $binDir to your PATH (restart open terminals to pick it up)"
}
if (($env:Path -split ';') -notcontains $binDir) { $env:Path = "$env:Path;$binDir" }
