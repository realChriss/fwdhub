$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

function Ok($msg) {
    Write-Host '  ' -NoNewline
    Write-Host ([char]0x2713) -ForegroundColor Green -NoNewline
    Write-Host " $msg"
}

$arch = if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$asset = "fwdhub-windows-$arch.exe"
$dir = "$env:LOCALAPPDATA\Microsoft\WindowsApps"
$exe = "$dir\fwdhub.exe"

Write-Host ''
Write-Host '  fwdhub installer' -ForegroundColor White
Write-Host ''

try {
    Invoke-WebRequest -UseBasicParsing "https://github.com/realChriss/fwdhub/releases/latest/download/$asset" -OutFile "$exe.new"
} catch {
    Write-Host '  ' -NoNewline
    Write-Host 'x' -ForegroundColor Red -NoNewline
    Write-Host " Download failed: $($_.Exception.Message)"
    Write-Host ''
    return
}
Ok ("Downloaded $asset ({0:N1} MB)" -f ((Get-Item "$exe.new").Length / 1MB))
Remove-Item "$exe.old" -ErrorAction SilentlyContinue
if (Test-Path $exe) { Move-Item $exe "$exe.old" }
Move-Item "$exe.new" $exe
Ok "Installed to $exe"
Write-Host ''
Write-Host '  Run ' -NoNewline
Write-Host 'fwdhub' -ForegroundColor Cyan -NoNewline
Write-Host ' to start.'
Write-Host ''
