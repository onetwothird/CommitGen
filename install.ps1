# Install CommitGen on Windows.
#
#   irm https://raw.githubusercontent.com/onetwothird/CommitGen/main/install.ps1 | iex
#
# Downloads the latest release into %LOCALAPPDATA%\Programs\CommitGen and adds
# it to your user PATH. Falls back to `go install` when no release binary is
# available.
$ErrorActionPreference = 'Stop'
$repo = 'onetwothird/CommitGen'
$dir  = if ($env:COMMITGEN_INSTALL_DIR) { $env:COMMITGEN_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\CommitGen' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }

$url = $null
try {
    $rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $url = ($rel.assets | Where-Object { $_.name -like "*_windows_$arch.zip" } | Select-Object -First 1).browser_download_url
} catch { }

if (-not $url) {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host 'No release binary found; building with Go instead...'
        go install "github.com/$repo@latest"
        Write-Host "Installed to $(go env GOPATH)\bin\CommitGen.exe"
        return
    }
    throw "No release binary found and Go is not installed. Install Go from https://go.dev/dl/ and run: go install github.com/$repo@latest"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("CommitGen-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force $tmp | Out-Null
try {
    Write-Host "Downloading $url"
    Invoke-WebRequest $url -OutFile (Join-Path $tmp 'CommitGen.zip')
    Expand-Archive (Join-Path $tmp 'CommitGen.zip') -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Force $dir | Out-Null
    $exe = Get-ChildItem $tmp -Filter '*.exe' -Recurse | Where-Object { $_.Name -ieq 'CommitGen.exe' } | Select-Object -First 1
    Copy-Item $exe.FullName (Join-Path $dir 'CommitGen.exe') -Force
} finally {
    Remove-Item -Recurse -Force $tmp
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
    Write-Host "Added $dir to your PATH. Open a new terminal to use it."
}
Write-Host "Installed $dir\CommitGen.exe"
Write-Host "Next: run 'CommitGen setup' (or just 'CommitGen' for offline mode)."
