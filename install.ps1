$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$arch = if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$dir = "$env:LOCALAPPDATA\Microsoft\WindowsApps"
$exe = "$dir\fwdhub.exe"

Write-Host "Downloading fwdhub-windows-$arch.exe..."
Invoke-WebRequest -UseBasicParsing "https://github.com/realChriss/fwdhub/releases/latest/download/fwdhub-windows-$arch.exe" -OutFile "$exe.new"
Remove-Item "$exe.old" -ErrorAction SilentlyContinue
if (Test-Path $exe) { Move-Item $exe "$exe.old" }
Move-Item "$exe.new" $exe
Write-Host "Installed $exe. Run: fwdhub"
